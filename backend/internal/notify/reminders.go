package notify

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/clevercode/sempa/internal/db"
)

// StartReminders runs a background loop that (1) fires per-task hard reminders as
// they come due and (2) sends a morning digest. All delivery goes through the
// Dispatcher, which honors the user's channel toggles. The loop is cheap (one
// indexed query per minute) and exits when ctx is cancelled.
func StartReminders(ctx context.Context, tasks *db.TaskStore, dispatcher *Dispatcher, configs *db.IntegrationConfigStore) {
	slog.Info("notify: reminder scheduler started")

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	var lastDigestDate string

	tick := func() {
		st := LoadSettings(ctx, configs)
		if !st.MasterEnabled {
			return
		}

		// 1. Per-task hard reminders.
		checkDueReminders(ctx, tasks, dispatcher)

		// 2. Morning digest, once per day at the configured hour.
		now := time.Now()
		today := now.Format("2006-01-02")
		if st.MorningDigest && now.Hour() == st.DigestHour && lastDigestDate != today {
			sendMorningDigest(ctx, tasks, dispatcher, today)
			lastDigestDate = today
		}
	}

	// Run once promptly on startup so a reminder set for "a minute ago" while the
	// server was down still fires soon after boot.
	tick()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tick()
		}
	}
}

func checkDueReminders(ctx context.Context, tasks *db.TaskStore, dispatcher *Dispatcher) {
	due, err := tasks.ListDueReminders(ctx)
	if err != nil {
		slog.Error("notify: list due reminders", "err", err)
		return
	}
	for _, t := range due {
		dispatcher.Send(ctx, Notification{
			Title:  "Reminder",
			Body:   t.Title,
			URL:    "/focus/" + t.ID,
			TaskID: t.ID,
			Tag:    "reminder-" + t.ID,
			Type:   "reminder",
		})
		if err := tasks.MarkReminderSent(ctx, t.ID); err != nil {
			slog.Error("notify: mark reminder sent", "id", t.ID, "err", err)
		} else {
			slog.Info("notify: sent task reminder", "id", t.ID)
		}
	}
}

func sendMorningDigest(ctx context.Context, tasks *db.TaskStore, dispatcher *Dispatcher, today string) {
	dayTasks, err := tasks.ListByDate(ctx, today)
	if err != nil {
		slog.Error("notify: list today tasks", "err", err)
		return
	}

	pending := 0
	for _, t := range dayTasks {
		if t.Status != "done" && t.Status != "cancelled" {
			pending++
		}
	}
	if pending == 0 {
		return
	}

	dispatcher.Send(ctx, Notification{
		Title: "Good morning",
		Body:  fmt.Sprintf("You have %d task%s planned for today.", pending, plural(pending)),
		URL:   "/home",
		Tag:   "morning-digest-" + today,
		Type:  "morning_digest",
	})
	slog.Info("notify: sent morning digest", "tasks", pending)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
