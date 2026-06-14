package poller

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/clevercode/sempa/internal/db"
)

// weeksAhead is how far past the current week the poller materialises recurring
// instances: current week + the next 2 weeks (~3 weeks of forward visibility).
const recurrenceWeeksAhead = 2

// StartRecurrence proactively materialises recurring task instances for the
// current week and the next few weeks. Offline-first clients read tasks from
// their local SQLite DB and never trigger the server's lazy, per-week
// generation, so future occurrences would otherwise never appear after a sync.
//
// It runs an initial pass shortly after boot, then every 6 hours — frequent
// enough to roll the horizon forward across midnight without precise timing.
// Generation is idempotent, so re-running is cheap and duplicate-free.
func StartRecurrence(ctx context.Context, database *sql.DB) {
	go func() {
		select {
		case <-time.After(20 * time.Second): // let startup migrations settle
		case <-ctx.Done():
			return
		}
		pollRecurrence(ctx, database)

		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pollRecurrence(ctx, database)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func pollRecurrence(ctx context.Context, database *sql.DB) {
	store := db.NewTaskStore(database)
	if err := store.GenerateHorizon(ctx, "", recurrenceWeeksAhead); err != nil {
		slog.Error("recurrence scheduler: generation failed", "err", err)
		return
	}
	slog.Info("recurrence scheduler: horizon generated", "weeks_ahead", recurrenceWeeksAhead)
}
