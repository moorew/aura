package db

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GenerateForDate runs the daily "smart rollover" for `date` (YYYY-MM-DD) and
// makes sure every template due that day has a fresh instance.
//
// Smart rollover — applied to every recurring instance still open *before* `date`:
//
//	pristine  (untouched: not customised, not started, no logged time)
//	          → deleted; today's fresh instance takes its place (no pile-up).
//	modified  (customised, in-progress, or with logged time)
//	          → carried forward to `date` (and its week_start realigned), so the
//	            user keeps their notes/subtasks and sees it alongside today's new
//	            instance.
//
// Pristine vs. modified is tracked by tasks.is_customized (set whenever the user
// edits an instance's content or adds a sub-task — see the API layer).
func (s *TaskStore) GenerateForDate(ctx context.Context, date string) error {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return fmt.Errorf("invalid date %q: %w", date, err)
	}
	ws := weekStartOf(t)

	// 1a. Delete pristine, open instances left in the past — nothing the user
	//     cared about, so today's fresh instance replaces them.
	if _, err := s.db.ExecContext(ctx, `
		DELETE FROM tasks
		WHERE recurrence_origin_id IS NOT NULL
		  AND is_customized = 0
		  AND status IN ('backlog','planned')
		  AND (time_actual_minutes IS NULL OR time_actual_minutes = 0)
		  AND planned_date IS NOT NULL
		  AND planned_date < ?`, date); err != nil {
		return err
	}

	// 1b. Whatever past, open recurring instances remain are "modified" — carry
	//     them forward to today and realign week_start so ListByWeek finds them.
	if _, err := s.db.ExecContext(ctx, `
		UPDATE tasks
		SET planned_date = ?, week_start = ?, updated_at = datetime('now')
		WHERE recurrence_origin_id IS NOT NULL
		  AND status IN ('backlog','planned','in_progress')
		  AND planned_date IS NOT NULL
		  AND planned_date < ?`, date, ws, date); err != nil {
		return err
	}

	// 2. Ensure a fresh instance exists for every template due on `date`. A
	//    carried-forward modified instance (is_customized = 1) does not count, so
	//    the user sees both it and the new pristine one.
	templates, err := s.ListRecurringTemplates(ctx)
	if err != nil {
		return err
	}
	for _, tmpl := range templates {
		if tmpl.RecurrenceRule == nil || !isDueOn(*tmpl.RecurrenceRule, t) {
			continue
		}
		if s.pristineInstanceExistsForDate(ctx, tmpl.ID, date) {
			continue
		}
		if err := s.createInstance(ctx, tmpl, t); err != nil {
			return err
		}
	}
	return nil
}

// GenerateForWeek ensures the requested week has the right recurring instances.
// `today` is the caller's local date (YYYY-MM-DD); pass "" to use the server's.
// Passing the client's date keeps rollover correct across timezones.
//
// This is intentionally non-destructive: it never deletes future instances (an
// earlier version did, which could race with the day/today view and make tasks
// vanish). Pristine-existence checks keep it idempotent and duplicate-free.
func (s *TaskStore) GenerateForWeek(ctx context.Context, weekStart, today string) error {
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}
	ws, err := time.Parse("2006-01-02", weekStart)
	if err != nil {
		return fmt.Errorf("invalid weekStart %q: %w", weekStart, err)
	}
	weekEnd := ws.AddDate(0, 0, 6).Format("2006-01-02")

	// Backfill: repair any task whose week_start is missing or doesn't match its
	// planned_date, so it surfaces in ListByWeek (which filters on week_start).
	s.db.ExecContext(ctx, `
		UPDATE tasks
		SET week_start = date(planned_date, '-' || ((CAST(strftime('%w', planned_date) AS INTEGER) + 6) % 7) || ' days')
		WHERE planned_date IS NOT NULL
		  AND (week_start IS NULL OR week_start = ''
		       OR week_start != date(planned_date, '-' || ((CAST(strftime('%w', planned_date) AS INTEGER) + 6) % 7) || ' days'))`)

	switch {
	case today > weekEnd:
		// Past week — instances are settled; nothing to do.
		return nil
	case today < weekStart:
		// Future week — seed every due day, no rollover.
		return s.seedWeekInstances(ctx, ws, "")
	default:
		// Current week — roll today over, then seed the remaining days.
		if err := s.GenerateForDate(ctx, today); err != nil {
			return err
		}
		return s.seedWeekInstances(ctx, ws, today)
	}
}

// GenerateHorizon ensures recurring instances exist for the current week and
// the next `weeksAhead` weeks. Offline-first clients (Tauri desktop, Capacitor
// Android) read tasks straight from their local SQLite DB and never hit the
// HTTP list endpoint that lazily generates instances, so without this the
// series appears to "end" at the last week a web client happened to request.
// Run proactively by the recurrence poller, these instances flow to every
// client through normal sync. `today` is YYYY-MM-DD; pass "" for the server's
// date. It is idempotent (pristine-existence checks keep it duplicate-free).
func (s *TaskStore) GenerateHorizon(ctx context.Context, today string, weeksAhead int) error {
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}
	t, err := time.Parse("2006-01-02", today)
	if err != nil {
		return fmt.Errorf("invalid today %q: %w", today, err)
	}
	curWS, err := time.Parse("2006-01-02", weekStartOf(t))
	if err != nil {
		return err
	}
	for i := 0; i <= weeksAhead; i++ {
		ws := curWS.AddDate(0, 0, i*7).Format("2006-01-02")
		if err := s.GenerateForWeek(ctx, ws, today); err != nil {
			return err
		}
	}
	return nil
}

// seedWeekInstances creates a pristine instance for each due day in the week
// (Mon–Sun) strictly after `afterDate` (use "" to include all days). Days that
// already have a pristine instance for the template are skipped.
func (s *TaskStore) seedWeekInstances(ctx context.Context, ws time.Time, afterDate string) error {
	templates, err := s.ListRecurringTemplates(ctx)
	if err != nil {
		return err
	}
	for i := 0; i < 7; i++ {
		d := ws.AddDate(0, 0, i)
		date := d.Format("2006-01-02")
		if afterDate != "" && date <= afterDate {
			continue
		}
		for _, tmpl := range templates {
			if tmpl.RecurrenceRule == nil || !isDueOn(*tmpl.RecurrenceRule, d) {
				continue
			}
			if s.pristineInstanceExistsForDate(ctx, tmpl.ID, date) {
				continue
			}
			if err := s.createInstance(ctx, tmpl, d); err != nil {
				return err
			}
		}
	}
	return nil
}

// createInstance materialises one recurring instance for the given template on
// day `t`, copying the template's roughly_at sort hint.
func (s *TaskStore) createInstance(ctx context.Context, tmpl Task, t time.Time) error {
	date := t.Format("2006-01-02")
	ws := weekStartOf(t)
	_, err := s.Create(ctx, CreateTaskParams{
		ID:                 uuid.New().String(),
		Title:              tmpl.Title,
		Description:        tmpl.Description,
		PlannedDate:        &date,
		WeekStart:          &ws,
		Status:             "planned",
		Position:           float64(t.UnixMilli()),
		Tags:               tmpl.Tags,
		RecurrenceOriginID: &tmpl.ID,
		RoughlyAt:          tmpl.RoughlyAt,
	})
	return err
}

// pristineInstanceExistsForDate reports whether a non-customised, non-cancelled
// instance of the template already exists on the given date.
func (s *TaskStore) pristineInstanceExistsForDate(ctx context.Context, originID, date string) bool {
	var count int
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks
		 WHERE recurrence_origin_id = ? AND planned_date = ?
		   AND is_customized = 0 AND status != 'cancelled'`,
		originID, date).Scan(&count)
	return count > 0
}

// weekStartOf returns the Monday-based week-start date (YYYY-MM-DD) for t,
// matching the frontend's weekStart() convention so recurring instances are
// found by ListByWeek (which filters on week_start).
func weekStartOf(t time.Time) string {
	offset := int(t.Weekday()) - int(time.Monday) // Sun=0 → -1, Mon=1 → 0, …
	if offset < 0 {
		offset += 7
	}
	return t.AddDate(0, 0, -offset).Format("2006-01-02")
}

// isDueOn reports whether the recurrence rule fires on date t.
//
// Supported rules:
//
//	"daily"          – every day
//	"weekdays"       – Mon–Fri
//	"weekends"       – Sat–Sun
//	"weekly:N"       – weekday N (0=Sun … 6=Sat)
//	"weekly:N,N,…"   – multiple weekdays
//	"monthly:D"      – day D of each month (capped to last day)
func isDueOn(rule string, t time.Time) bool {
	switch {
	case rule == "daily":
		return true
	case rule == "weekdays":
		wd := t.Weekday()
		return wd >= time.Monday && wd <= time.Friday
	case rule == "weekends":
		wd := t.Weekday()
		return wd == time.Saturday || wd == time.Sunday
	case strings.HasPrefix(rule, "weekly:"):
		days := strings.Split(strings.TrimPrefix(rule, "weekly:"), ",")
		wd := int(t.Weekday())
		for _, d := range days {
			if n, err := strconv.Atoi(strings.TrimSpace(d)); err == nil && n == wd {
				return true
			}
		}
		return false
	case strings.HasPrefix(rule, "monthly:"):
		n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(rule, "monthly:")))
		if err != nil {
			return false
		}
		lastDay := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
		if n > lastDay {
			n = lastDay
		}
		return t.Day() == n
	}
	return false
}
