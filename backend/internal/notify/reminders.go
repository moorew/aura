package notify

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/clevercode/sempa/internal/db"
)

// StartReminders runs a background loop that checks for tasks needing reminders.
// It sends a morning digest and alerts for overdue tasks.
func StartReminders(ctx context.Context, tasks *db.TaskStore, svc *Service) {
	if !svc.Enabled() {
		slog.Info("notify: reminders disabled (no FCM key configured)")
		return
	}

	slog.Info("notify: reminder scheduler started")

	// Check every 15 minutes
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	// Track what we've sent today to avoid duplicates
	var lastDigestDate string

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			today := now.Format("2006-01-02")
			hour := now.Hour()

			// Morning digest at 8 AM
			if hour == 8 && lastDigestDate != today {
				sendMorningDigest(tasks, svc, today)
				lastDigestDate = today
			}
		}
	}
}

func sendMorningDigest(tasks *db.TaskStore, svc *Service, today string) {
	dayTasks, err := tasks.ListByDate(context.Background(), today)
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

	title := "Good morning"
	body := fmt.Sprintf("You have %d task%s planned for today.", pending, plural(pending))

	svc.SendToAll(title, body, map[string]string{
		"type": "morning_digest",
		"date": today,
	})

	slog.Info("notify: sent morning digest", "tasks", pending)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
