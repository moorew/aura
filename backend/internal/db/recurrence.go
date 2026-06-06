package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GenerateForDate generates recurring task instances for the given date (YYYY-MM-DD).
//
// Rollover rule: if an uncompleted, non-customised instance exists on or before
// `date`, move it forward to `date`.  If none exists and nothing already covers
// this date, create a fresh instance.
func (s *TaskStore) GenerateForDate(ctx context.Context, date string) error {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return fmt.Errorf("invalid date %q: %w", date, err)
	}

	templates, err := s.ListRecurringTemplates(ctx)
	if err != nil {
		return err
	}

	for _, tmpl := range templates {
		if tmpl.RecurrenceRule == nil || !isDueOn(*tmpl.RecurrenceRule, t) {
			continue
		}

		// Skip if this date already has a live instance.
		if s.recurringInstanceExistsForDate(ctx, tmpl.ID, date) {
			continue
		}

		// Look for an uncompleted instance on or before `date` (never future).
		pending, err := s.findPendingRecurringInstanceBefore(ctx, tmpl.ID, date)
		if err != nil {
			return err
		}

		if pending != nil {
			pending.PlannedDate = &date
			if _, err := s.Update(ctx, *pending); err != nil {
				return err
			}
		} else {
			ws := weekStartOf(t)
			if _, err := s.Create(ctx, CreateTaskParams{
				ID:                 uuid.New().String(),
				Title:              tmpl.Title,
				Description:        tmpl.Description,
				PlannedDate:        &date,
				WeekStart:          &ws,
				Status:             "planned",
				Position:           float64(t.UnixMilli()),
				Tags:               tmpl.Tags,
				RecurrenceOriginID: &tmpl.ID,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// GenerateForWeek ensures each day of the requested week has the right recurring
// instances:
//
//   - Past weeks: no-op (history is settled).
//   - Current week: delete stale uncompleted future instances, roll today's task
//     forward via GenerateForDate, then seed fresh planned instances for every
//     upcoming day in the week.
//   - Future weeks: seed one planned instance per due day, no rollover.
func (s *TaskStore) GenerateForWeek(ctx context.Context, weekStart string) error {
	today := time.Now().Format("2006-01-02")
	ws, err := time.Parse("2006-01-02", weekStart)
	if err != nil {
		return fmt.Errorf("invalid weekStart %q: %w", weekStart, err)
	}
	weekEnd := ws.AddDate(0, 0, 6).Format("2006-01-02")

	// Backfill: repair any tasks that have a planned_date but a missing
	// week_start (e.g. recurring instances created before week_start was set on
	// generation). Without this they'd never surface in ListByWeek. Computes the
	// Monday-based week start to match the frontend convention.
	s.db.ExecContext(ctx, `
		UPDATE tasks
		SET week_start = date(planned_date, '-' || ((CAST(strftime('%w', planned_date) AS INTEGER) + 6) % 7) || ' days')
		WHERE planned_date IS NOT NULL AND (week_start IS NULL OR week_start = '')`)

	switch {
	case today > weekEnd:
		// Past week — nothing to do; instances are already settled.
		return nil

	case today < weekStart:
		// Future week — seed planned instances without rollover.
		return s.seedWeekInstances(ctx, ws, "")

	default:
		// Current week.
		// 1. Wipe uncompleted non-customised instances for the rest of the week
		//    so they can be re-seeded fresh (prevents stale accumulation).
		s.db.ExecContext(ctx, `
			DELETE FROM tasks
			WHERE recurrence_origin_id IS NOT NULL
			  AND is_customized = 0
			  AND status IN ('backlog','planned')
			  AND planned_date > ?
			  AND planned_date <= ?`,
			today, weekEnd)

		// 2. Roll today's task forward (or create if none exists).
		if err := s.GenerateForDate(ctx, today); err != nil {
			return err
		}

		// 3. Seed one fresh instance per due day for the rest of the week.
		return s.seedWeekInstances(ctx, ws, today)
	}
}

// seedWeekInstances creates planned instances for each day in the week (Mon–Sun)
// that is strictly after `afterDate` (use "" to include all days).
// Skips days that already have a live instance for the template.
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
			if s.recurringInstanceExistsForDate(ctx, tmpl.ID, date) {
				continue
			}
			planDate := date
			ws := weekStartOf(d)
			if _, err := s.Create(ctx, CreateTaskParams{
				ID:                 uuid.New().String(),
				Title:              tmpl.Title,
				Description:        tmpl.Description,
				PlannedDate:        &planDate,
				WeekStart:          &ws,
				Status:             "planned",
				Position:           float64(d.UnixMilli()),
				Tags:               tmpl.Tags,
				RecurrenceOriginID: &tmpl.ID,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// findPendingRecurringInstanceBefore finds the most recent uncompleted,
// non-customised instance of a template with planned_date <= maxDate.
// This prevents future pre-seeded instances from interfering with rollover.
func (s *TaskStore) findPendingRecurringInstanceBefore(ctx context.Context, originID, maxDate string) (*Task, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+taskCols+` FROM tasks
		 WHERE recurrence_origin_id = ?
		   AND status IN ('backlog','planned')
		   AND is_customized = 0
		   AND planned_date <= ?
		 ORDER BY planned_date DESC
		 LIMIT 1`, originID, maxDate)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// recurringInstanceExistsForDate returns true if a non-cancelled instance of
// the given template already exists on the given date.
func (s *TaskStore) recurringInstanceExistsForDate(ctx context.Context, originID, date string) bool {
	var count int
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks
		 WHERE recurrence_origin_id = ? AND planned_date = ? AND status != 'cancelled'`,
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
