package poller

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/clevercode/sempa/internal/db"
	"github.com/clevercode/sempa/internal/integrations/fastmail"
)

// StartInbox polls the task_inbox integration on the given interval.
func StartInbox(ctx context.Context, database *sql.DB, interval time.Duration, ollamaBaseURL, ollamaModel string) {
	if interval < 30*time.Second {
		interval = 30 * time.Second
	}
	go func() {
		pollInbox(ctx, database, ollamaBaseURL, ollamaModel)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pollInbox(ctx, database, ollamaBaseURL, ollamaModel)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func pollInbox(ctx context.Context, database *sql.DB, ollamaBaseURL, ollamaModel string) {
	configs := db.NewIntegrationConfigStore(database)
	tasks := db.NewTaskStore(database)

	cfg, err := configs.Get(ctx, "task_inbox")
	if err != nil {
		return
	}

	var inboxCfg fastmail.InboxConfig
	if err := json.Unmarshal([]byte(cfg.Config), &inboxCfg); err != nil {
		slog.Error("inbox poller: bad config", "err", err)
		return
	}
	inboxCfg.OllamaBaseURL = ollamaBaseURL
	inboxCfg.OllamaModel = ollamaModel

	result, err := fastmail.SyncTaskInbox(ctx, inboxCfg, tasks)
	if err != nil {
		slog.Error("inbox poller: sync failed", "err", err)
		return
	}
	if result.New > 0 || result.Errors > 0 {
		slog.Info("inbox poller: sync complete", "new", result.New, "errors", result.Errors)
	}
	_ = configs.TouchSyncTime(ctx, "task_inbox")
}
