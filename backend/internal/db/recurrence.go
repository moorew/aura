package db

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GenerateForDate ensures exactly one planned instance of each due recurring
// template exists on `date`.  It never moves instances — if a template is due
// on a day, one instance is created there and stays there regardless of whether
// earlier instances were completed.
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
		if s.recurringInstanceExistsForDate(ctx, tmpl.ID, date) {
			continue
		}
		if _, err := s.Create(ctx, CreateTaskParams{
			ID:                 uuid.New().String(),
			Title:              tmpl.Title,
			Description:        tmpl.Description,
			PlannedDate:        &date,
			Status:             "planned",
			Position:           float64(t.UnixMilli()),
			Tags:               tmpl.Tags,
			RecurrenceOriginID: &tmpl.ID,
		}); err != nil {
			return err
		}
	}
	return nil
}

// GenerateForWeek ensures one instance per due day exists for every recurring
// template across all 7 days of the week.
func (s *TaskStore) GenerateForWeek(ctx context.Context, weekStart string) error {
	ws, err := time.Parse("2006-01-02", weekStart)
	if err != nil {
		return fmt.Errorf("invalid weekStart %q: %w", weekStart, err)
	}
	for i := 0; i < 7; i++ {
		date := ws.AddDate(0, 0, i).Format("2006-01-02")
		if err := s.GenerateForDate(ctx, date); err != nil {
			return err
		}
	}
	return nil
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
