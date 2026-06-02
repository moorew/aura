package config

import "os"

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

	// Auth — optional; if AuthPassword is empty, auth is disabled
	AuthUsername string
	AuthPassword string

	// Webhook token for Cloudflare Email Routing → POST /api/v1/tasks/from-email
	EmailForwardToken string
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
		EmailForwardToken: env("EMAIL_FORWARD_TOKEN", ""),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
