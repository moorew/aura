package poller

import (
	"context"
	"log/slog"
	"time"

	"github.com/clevercode/sempa/internal/backup"
	"github.com/clevercode/sempa/internal/db"
)

// StartBackupScheduler runs a daily backup at the configured local hour. It
// wakes every 10 minutes and fires once per day, after the configured hour,
// when backups are enabled.
func StartBackupScheduler(ctx context.Context, svc *backup.Service, store *db.BackupStore, driveToken backup.DriveTokenFunc) {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		var lastRunDay string
		select {
		case <-time.After(30 * time.Second): // let startup settle
		case <-ctx.Done():
			return
		}
		for {
			lastRunDay = maybeRunBackup(ctx, svc, store, driveToken, lastRunDay)
			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
		}
	}()
}

func maybeRunBackup(ctx context.Context, svc *backup.Service, store *db.BackupStore, driveToken backup.DriveTokenFunc, lastRunDay string) string {
	settings, err := store.Get(ctx)
	if err != nil || !settings.Enabled {
		return lastRunDay
	}
	now := time.Now()
	today := now.Format("2006-01-02")
	if today == lastRunDay { // already ran today
		return lastRunDay
	}
	if now.Hour() < settings.ScheduleHour { // not yet time
		return lastRunDay
	}
	slog.Info("backup scheduler: starting daily backup")
	run, err := svc.Run(ctx, "scheduled", driveToken)
	if err != nil {
		slog.Error("backup scheduler: run failed", "err", err, "run_id", run.ID)
	} else {
		slog.Info("backup scheduler: completed", "run_id", run.ID)
	}
	return today
}
