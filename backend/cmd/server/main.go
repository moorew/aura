package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/clevercode/sempa/internal/api"
	"github.com/clevercode/sempa/internal/config"
	"github.com/clevercode/sempa/internal/db"
	"github.com/clevercode/sempa/internal/integrations/emailrecv"
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

	// Cancellable context for background workers.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := api.NewRouter(database, cfg)

	// Inbound SMTP server.
	if cfg.SMTPPort != "" {
		smtpSrv := emailrecv.New(":"+cfg.SMTPPort, db.NewTaskStore(database))
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

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
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
