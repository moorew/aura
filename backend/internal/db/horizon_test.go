package db

import (
	"context"
	"testing"
)

// A daily template with today = Sat 2026-06-13 must, after GenerateHorizon,
// have instances every day across the current week's remaining days AND the
// next two full weeks — i.e. it must not "end tomorrow".
func TestGenerateHorizonDailyCoversFutureWeeks(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	origin := makeDailyTemplate(t, s, nil)

	const today = "2026-06-13" // Saturday
	if err := s.GenerateHorizon(ctx, today, 2); err != nil {
		t.Fatalf("GenerateHorizon: %v", err)
	}

	all, err := s.ListByRecurrenceOrigin(ctx, origin)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	have := map[string]bool{}
	for _, task := range all {
		if task.PlannedDate != nil && task.Status != "cancelled" {
			have[*task.PlannedDate] = true
		}
	}

	// today + tomorrow (current week) and every day of the next two weeks.
	want := []string{
		"2026-06-13", "2026-06-14", // current week (rest was rolled over)
		"2026-06-15", "2026-06-16", "2026-06-17", "2026-06-18", "2026-06-19", "2026-06-20", "2026-06-21", // next week
		"2026-06-22", "2026-06-23", "2026-06-24", "2026-06-25", "2026-06-26", "2026-06-27", "2026-06-28", // week after
	}
	for _, d := range want {
		if !have[d] {
			t.Errorf("missing daily instance on %s", d)
		}
	}

	// Idempotent: a second pass must not create duplicates.
	if err := s.GenerateHorizon(ctx, today, 2); err != nil {
		t.Fatalf("GenerateHorizon (2nd): %v", err)
	}
	all2, _ := s.ListByRecurrenceOrigin(ctx, origin)
	perDay := map[string]int{}
	for _, task := range all2 {
		if task.PlannedDate != nil && task.Status != "cancelled" {
			perDay[*task.PlannedDate]++
		}
	}
	for d, n := range perDay {
		if n != 1 {
			t.Errorf("duplicate instances on %s: got %d", d, n)
		}
	}
}
