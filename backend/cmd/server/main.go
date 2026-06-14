package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/api"
	"github.com/clevercode/sempa/internal/backup"
	"github.com/clevercode/sempa/internal/blob"
	"github.com/clevercode/sempa/internal/config"
	"github.com/clevercode/sempa/internal/db"
	"github.com/clevercode/sempa/internal/integrations/emailrecv"
	"github.com/clevercode/sempa/internal/notify"
	"github.com/clevercode/sempa/internal/poller"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}

	blobs, err := blob.New(cfg.AttachmentsDir)
	if err != nil {
		slog.Error("open attachments dir", "err", err)
		os.Exit(1)
	}

	// Cancellable context for background workers.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Web Push (VAPID) key pair — generated once and persisted in the DB so the
	// browser's stored subscriptions stay valid across restarts.
	configStore := db.NewIntegrationConfigStore(database)
	vapidKeys, err := loadOrCreateVAPID(ctx, configStore)
	if err != nil {
		slog.Error("vapid keys", "err", err)
		os.Exit(1)
	}

	handler := api.NewRouter(database, cfg, blobs, vapidKeys.Public)

	// Inbound SMTP server.
	if cfg.SMTPPort != "" {
		smtpSrv := emailrecv.New(":"+cfg.SMTPPort, db.NewTaskStore(database), cfg.SMTPAllowedSenders)
		go func() {
			slog.Info("smtp listening", "port", cfg.SMTPPort)
			if err := smtpSrv.ListenAndServe(); err != nil {
				slog.Error("smtp server error", "err", err)
			}
		}()
	}

	// Background inbox poller.
	if cfg.InboxPollInterval != "" {
		interval, err := time.ParseDuration(cfg.InboxPollInterval)
		if err != nil {
			slog.Warn("invalid INBOX_POLL_INTERVAL, using 5m", "value", cfg.InboxPollInterval)
			interval = 5 * time.Minute
		}
		poller.StartInbox(ctx, database, interval, cfg.OllamaBaseURL, cfg.OllamaModel)
		slog.Info("inbox poller started", "interval", interval)
	}

	// Background calendar refresh — keeps ICS subscriptions and the Fastmail
	// calendar fresh (and timezone-correct) without manual syncing.
	if cfg.CalendarPollInterval != "" {
		interval, err := time.ParseDuration(cfg.CalendarPollInterval)
		if err != nil {
			slog.Warn("invalid CALENDAR_POLL_INTERVAL, using 15m", "value", cfg.CalendarPollInterval)
			interval = 15 * time.Minute
		}
		poller.StartCalendars(ctx, database, interval)
		slog.Info("calendar poller started", "interval", interval)
	}

	// Push notification reminders. The dispatcher fans messages out to Web Push,
	// FCM and the generic webhook, honoring the user's channel toggles.
	fcmSvc := notify.New(db.NewDeviceTokenStore(database), cfg.FCMKeyPath)
	webPush := notify.NewWebPushSender(vapidKeys, cfg.VAPIDSubject)
	dispatcher := notify.NewDispatcher(configStore, db.NewPushSubStore(database), webPush, fcmSvc, cfg.AppURL)
	go notify.StartReminders(ctx, db.NewTaskStore(database), dispatcher, configStore)

	// Daily backup scheduler.
	backupSvc := backup.NewService(database, cfg.DBPath, blobs.Dir())
	driveToken := backup.DriveTokenResolver(db.NewIntegrationConfigStore(database), cfg.GmailClientID, cfg.GmailClientSecret)
	poller.StartBackupScheduler(ctx, backupSvc, db.NewBackupStore(database), driveToken)
	slog.Info("backup scheduler started")

	// Recurring-task horizon. Materialises future recurring instances so
	// offline-first clients (which read the local DB and never trigger the
	// lazy per-week generation) see them after a sync.
	poller.StartRecurrence(ctx, database)
	slog.Info("recurrence scheduler started")

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
		// ReadTimeout/WriteTimeout are left at 0 (unlimited) so large attachment
		// uploads and backup downloads (up to 500 MB) aren't cut off. Slowloris is
		// still guarded by ReadHeaderTimeout.
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("listening", "addr", srv.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down...")
	cancel() // stop background workers
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
	slog.Info("stopped")
}

// loadOrCreateVAPID returns the persisted VAPID key pair, generating and saving
// a new one on first boot. Stored in integration_configs(type='webpush_vapid').
func loadOrCreateVAPID(ctx context.Context, store *db.IntegrationConfigStore) (notify.VAPIDKeys, error) {
	const vapidType = "webpush_vapid"
	if cfg, err := store.Get(ctx, vapidType); err == nil && cfg.Config != "" {
		var keys notify.VAPIDKeys
		if json.Unmarshal([]byte(cfg.Config), &keys) == nil && keys.Public != "" && keys.Private != "" {
			return keys, nil
		}
	}
	keys, err := notify.GenerateVAPIDKeys()
	if err != nil {
		return notify.VAPIDKeys{}, err
	}
	raw, _ := json.Marshal(keys)
	if _, err := store.Upsert(ctx, uuid.New().String(), vapidType, string(raw)); err != nil {
		return notify.VAPIDKeys{}, err
	}
	slog.Info("notify: generated new VAPID key pair")
	return keys, nil
}
