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
}

func Load() Config {
	return Config{
		Port:              env("PORT", "8080"),
		DBPath:            env("DB_PATH", "./data/aura.db"),
		Env:               env("ENV", "development"),
		FrontendDir:       env("FRONTEND_DIR", ""),
		AppURL:            env("APP_URL", "http://localhost:8080"),
		FrontendURL:       env("FRONTEND_URL", "http://localhost:5173"),
		GmailClientID:     env("GMAIL_CLIENT_ID", ""),
		GmailClientSecret: env("GMAIL_CLIENT_SECRET", ""),
		SMTPPort:          env("SMTP_PORT", "2525"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
