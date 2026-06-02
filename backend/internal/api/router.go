package api

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/clevercode/aura/internal/config"
	"github.com/clevercode/aura/internal/db"
)

func NewRouter(database *sql.DB, cfg config.Config) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	allowOrigin := func(_ *http.Request, origin string) bool {
		if cfg.Env != "production" {
			return strings.HasPrefix(origin, "http://localhost:") ||
				strings.HasPrefix(origin, "http://127.0.0.1:")
		}
		return origin == cfg.FrontendURL
	}
	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc:  allowOrigin,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	tagStore := db.NewTagStore(database)
	auth := newAuthHandler(cfg)

	tasks        := &taskHandler{store: db.NewTaskStore(database), tags: tagStore}
	objectives   := &objectiveHandler{store: db.NewObjectiveStore(database)}
	plans        := &planHandler{store: db.NewDailyPlanStore(database)}
	sessions     := &sessionHandler{store: db.NewSessionStore(database)}
	tags         := &tagHandler{store: tagStore}
	integrations := &integrationHandler{
		configs: db.NewIntegrationConfigStore(database),
		tasks:   db.NewTaskStore(database),
		cfg:     cfg,
	}

	r.Route("/api/v1", func(r chi.Router) {
		// Public: health + auth endpoints
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			respond(w, http.StatusOK, map[string]string{"status": "ok"})
		})
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", auth.login)
			r.Post("/logout", auth.logout)
			r.Get("/me", auth.me)
		})

		// Cloudflare email webhook — token-auth, not session-auth
		r.Post("/tasks/from-email", integrations.fromEmail)

		// Gmail OAuth callback must be accessible during the redirect flow
		r.Get("/integrations/gmail/callback", integrations.gmailCallback)

		// All remaining API routes require session auth (if auth is configured)
		r.Group(func(r chi.Router) {
			r.Use(auth.requireAuth)

			r.Route("/tasks", func(r chi.Router) {
				r.Get("/", tasks.list)
				r.Post("/", tasks.create)
				r.Get("/recurring", tasks.listTemplates)
				r.Get("/{id}", tasks.get)
				r.Patch("/{id}", tasks.update)
				r.Delete("/{id}", tasks.delete)
			})

			r.Route("/tags", func(r chi.Router) {
				r.Get("/", tags.list)
				r.Post("/", tags.create)
				r.Patch("/{id}", tags.update)
				r.Delete("/{id}", tags.delete)
			})

			r.Route("/objectives", func(r chi.Router) {
				r.Get("/", objectives.list)
				r.Post("/", objectives.create)
				r.Get("/{id}", objectives.get)
				r.Patch("/{id}", objectives.update)
				r.Delete("/{id}", objectives.delete)
			})

			r.Route("/plans", func(r chi.Router) {
				r.Get("/{date}", plans.get)
				r.Put("/{date}", plans.upsert)
			})

			r.Route("/pomodoros", func(r chi.Router) {
				r.Post("/", sessions.create)
			})

			r.Route("/integrations", func(r chi.Router) {
				r.Route("/jira", func(r chi.Router) {
					r.Get("/", integrations.jiraGet)
					r.Put("/", integrations.jiraPut)
					r.Delete("/", integrations.jiraDelete)
					r.Post("/test", integrations.jiraTest)
					r.Post("/sync", integrations.jiraSync)
				})
				r.Route("/gmail", func(r chi.Router) {
					r.Get("/", integrations.gmailGet)
					r.Delete("/", integrations.gmailDelete)
					r.Get("/auth", integrations.gmailAuth)
					r.Patch("/labels", integrations.gmailUpdateLabels)
					r.Post("/sync", integrations.gmailSync)
				})
				r.Route("/calendar", func(r chi.Router) {
					r.Get("/", integrations.calendarGet)
					r.Patch("/", integrations.calendarToggle)
					r.Post("/sync", integrations.calendarSync)
				})
				r.Route("/fastmail", func(r chi.Router) {
					r.Get("/", integrations.fastmailGet)
					r.Put("/", integrations.fastmailPut)
					r.Delete("/", integrations.fastmailDelete)
					r.Post("/sync", integrations.fastmailSync)
				})
				r.Get("/email-forward", integrations.emailForwardGet)
			})
		})
	})

	// Serve static frontend if configured (SPA fallback to index.html)
	if cfg.FrontendDir != "" {
		r.Handle("/*", spaHandler(cfg.FrontendDir))
	}

	return r
}

// spaHandler serves static files and falls back to index.html for client-side routing.
func spaHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})
}
