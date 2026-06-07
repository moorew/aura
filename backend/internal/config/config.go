package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Port           string
	DBPath         string
	AttachmentsDir string // dir for attachment blobs; default <db-dir>/attachments
	Env            string
	FrontendDir    string // path to built static frontend; empty = API-only mode

	// OAuth / integration
	AppURL            string // e.g. https://blackbox.clevercode.ts.net
	FrontendURL       string // e.g. http://localhost:5173 (dev only)
	GmailClientID     string
	GmailClientSecret string

	// Inbound SMTP (email forwarding)
	SMTPPort           string   // e.g. "2525"; empty disables the SMTP server
	SMTPAllowedSenders []string // email addresses or @domain; empty = accept all

	// Auth — optional; if AuthPassword is empty, password auth is disabled
	AuthUsername string
	AuthPassword string
	// Google Sign-In: comma-separated emails allowed to log in.
	// If empty, any Google account is accepted (fine for self-hosted on Tailscale).
	AllowedEmails []string

	// Webhook token for Cloudflare Email Routing → POST /api/v1/tasks/from-email
	EmailForwardToken string

	// Background inbox polling interval (e.g. "1m"); empty disables
	InboxPollInterval string

	// Optional: Ollama base URL for local AI-powered task title cleanup
	OllamaBaseURL string // e.g. http://ollama:11434
	OllamaModel   string // default: qwen2.5:1.5b

	// Firebase Cloud Messaging — path to service account JSON key file
	FCMKeyPath string // e.g. ./firebase-service-account.json
}

func Load() Config {
	dbPath := env("DB_PATH", "./data/sempa.db")
	return Config{
		Port:               env("PORT", "8080"),
		DBPath:             dbPath,
		AttachmentsDir:     env("ATTACHMENTS_DIR", filepath.Join(filepath.Dir(dbPath), "attachments")),
		Env:                env("ENV", "development"),
		FrontendDir:        env("FRONTEND_DIR", ""),
		AppURL:             env("APP_URL", "http://localhost:8080"),
		FrontendURL:        env("FRONTEND_URL", "http://localhost:5173"),
		GmailClientID:      env("GMAIL_CLIENT_ID", ""),
		GmailClientSecret:  env("GMAIL_CLIENT_SECRET", ""),
		SMTPPort:           env("SMTP_PORT", "2525"),
		SMTPAllowedSenders: splitEmails(env("SMTP_ALLOWED_SENDERS", "")),
		AuthUsername:       env("SEMPA_USERNAME", "admin"),
		AuthPassword:       env("SEMPA_PASSWORD", ""),
		AllowedEmails:      splitEmails(env("SEMPA_ALLOWED_EMAILS", "")),
		EmailForwardToken:  env("EMAIL_FORWARD_TOKEN", ""),
		InboxPollInterval:  env("INBOX_POLL_INTERVAL", "1m"),
		OllamaBaseURL:      env("OLLAMA_BASE_URL", ""),
		OllamaModel:        env("OLLAMA_MODEL", "qwen2.5:1.5b"),
		FCMKeyPath:         env("FCM_KEY_PATH", ""),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitEmails(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, strings.ToLower(v))
		}
	}
	return out
}
