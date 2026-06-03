package config

import (
	"os"
	"strings"
)

type Config struct {
	Port        string
	DBPath      string
	Env         string
	FrontendDir string // path to built static frontend; empty = API-only mode

	// OAuth / integration
	AppURL            string // e.g. https://blackbox.clevercode.ts.net
	FrontendURL       string // e.g. http://localhost:5173 (dev only)
	GmailClientID     string
	GmailClientSecret string

	// Inbound SMTP (email forwarding)
	SMTPPort string // e.g. "2525"; empty disables the SMTP server

	// Auth — optional; if AuthPassword is empty, password auth is disabled
	AuthUsername  string
	AuthPassword  string
	// Google Sign-In: comma-separated emails allowed to log in.
	// If empty, any Google account is accepted (fine for self-hosted on Tailscale).
	AllowedEmails []string

	// Webhook token for Cloudflare Email Routing → POST /api/v1/tasks/from-email
	EmailForwardToken string

	// Background inbox polling interval (e.g. "1m"); empty disables
	InboxPollInterval string

	// Optional: Anthropic API key for AI-powered task title cleanup
	AnthropicAPIKey string
}

func Load() Config {
	return Config{
		Port:              env("PORT", "8080"),
		DBPath:            env("DB_PATH", "./data/sempa.db"),
		Env:               env("ENV", "development"),
		FrontendDir:       env("FRONTEND_DIR", ""),
		AppURL:            env("APP_URL", "http://localhost:8080"),
		FrontendURL:       env("FRONTEND_URL", "http://localhost:5173"),
		GmailClientID:     env("GMAIL_CLIENT_ID", ""),
		GmailClientSecret: env("GMAIL_CLIENT_SECRET", ""),
		SMTPPort:          env("SMTP_PORT", "2525"),
		AuthUsername:      env("SEMPA_USERNAME", "admin"),
		AuthPassword:      env("SEMPA_PASSWORD", ""),
		AllowedEmails:     splitEmails(env("SEMPA_ALLOWED_EMAILS", "")),
		EmailForwardToken: env("EMAIL_FORWARD_TOKEN", ""),
		InboxPollInterval: env("INBOX_POLL_INTERVAL", "1m"),
		AnthropicAPIKey:   env("ANTHROPIC_API_KEY", ""),
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
