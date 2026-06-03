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

	"github.com/clevercode/sempa/internal/config"
	"github.com/clevercode/sempa/internal/db"
)

func NewRouter(database *sql.DB, cfg config.Config) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	allowOrigin := func(_ *http.Request, origin string) bool {
		if cfg.Env != "production" {
			return strings.HasPrefix(origin, "http://localhost") ||
				strings.HasPrefix(origin, "https://localhost") ||
				strings.HasPrefix(origin, "http://127.0.0.1:")
		}
		// Allow Capacitor mobile app origins
		if origin == "http://localhost" || origin == "https://localhost" || origin == "capacitor://localhost" {
			return true
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

	tagStore    := db.NewTagStore(database)
	configStore := db.NewIntegrationConfigStore(database)
	setup       := &setupHandler{configs: configStore}
	fmCalStore  := db.NewFastmailCalStore(database)
	auth := newAuthHandler(cfg)

	tasks        := &taskHandler{
		store:   db.NewTaskStore(database),
		tags:    tagStore,
		configs: configStore,
		appURL:  cfg.AppURL,
	}
	objectives   := &objectiveHandler{store: db.NewObjectiveStore(database)}
	plans        := &planHandler{store: db.NewDailyPlanStore(database)}
	sessions     := &sessionHandler{store: db.NewSessionStore(database)}
	tags         := &tagHandler{store: tagStore}
	weekReviews  := &weekReviewHandler{store: db.NewWeekReviewStore(database)}
	icals        := &icalHandler{
		store:      db.NewICalStore(database),
		fmCalStore: fmCalStore,
		configs:    configStore,
	}
	devices      := &deviceHandler{store: db.NewDeviceTokenStore(database)}
	integrations := &integrationHandler{
		configs:    configStore,
		tasks:      db.NewTaskStore(database),
		fmCalStore: fmCalStore,
		cfg:        cfg,
	}

	r.Route("/api/v1", func(r chi.Router) {
		// Public: health + auth endpoints
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			respond(w, http.StatusOK, map[string]string{"status": "ok"})
		})
		// Public auth config — lets the login page know which methods are available
		// before the user has a session. Always 200, never needs a cookie.
		r.Get("/auth/config", func(w http.ResponseWriter, r *http.Request) {
			respond(w, http.StatusOK, map[string]bool{
				"google_enabled":   auth.googleEnabled(),
				"password_enabled": auth.passwordEnabled(),
			})
		})
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", auth.login)
			r.Post("/logout", auth.logout)
			r.Get("/me", auth.me)
			r.Get("/google", auth.googleAuth)
			r.Get("/google/callback", auth.googleCallback)
		})

		// Setup status — public read so the frontend can redirect before auth
		r.Get("/setup/status", setup.status)

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

			r.Route("/weeks", func(r chi.Router) {
				r.Get("/{weekStart}/review", weekReviews.get)
				r.Put("/{weekStart}/review", weekReviews.upsert)
			})

			r.Route("/ical", func(r chi.Router) {
				r.Get("/subscriptions", icals.listSubscriptions)
				r.Post("/subscriptions", icals.createSubscription)
				r.Delete("/subscriptions/{id}", icals.deleteSubscription)
				r.Post("/subscriptions/{id}/sync", icals.syncSubscription)
				r.Get("/events", icals.listEventsForDate)
			})

			r.Post("/setup/complete", setup.complete)

			r.Route("/pomodoros", func(r chi.Router) {
				r.Get("/", sessions.listByTask)
				r.Post("/", sessions.create)
			})

			r.Route("/devices", func(r chi.Router) {
				r.Post("/", devices.register)
				r.Delete("/", devices.unregister)
			})

			r.Route("/integrations", func(r chi.Router) {
				r.Route("/jira", func(r chi.Router) {
					r.Get("/", integrations.jiraGet)
					r.Put("/", integrations.jiraPut)
					r.Delete("/", integrations.jiraDelete)
					r.Post("/test", integrations.jiraTest)
					r.Post("/sync", integrations.jiraSync)
					r.Get("/statuses", integrations.jiraGetStatuses)
					r.Get("/issues/{key}", integrations.jiraGetIssue)
					r.Get("/issues/{key}/transitions", integrations.jiraGetTransitions)
					r.Post("/issues/{key}/transition", integrations.jiraDoTransition)
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
					r.Get("/emails", integrations.fastmailEmails)
					r.Get("/emails/archived", integrations.fastmailArchivedEmails)
					r.Post("/emails/{id}/to-task", integrations.fastmailEmailToTask)
					r.Post("/emails/{id}/archive", integrations.fastmailArchiveEmail)
					r.Post("/emails/{id}/unarchive", integrations.fastmailUnarchiveEmail)
					r.Route("/calendar", func(r chi.Router) {
						r.Get("/", integrations.fastmailCalendarGet)
						r.Patch("/", integrations.fastmailCalendarToggle)
						r.Post("/sync", integrations.fastmailCalendarSync)
					})
				})
				r.Get("/email-forward", integrations.emailForwardGet)
			r.Route("/task-inbox", func(r chi.Router) {
				r.Get("/", integrations.taskInboxGet)
				r.Put("/", integrations.taskInboxPut)
				r.Patch("/senders", integrations.taskInboxPatchSenders)
				r.Post("/sync", integrations.taskInboxSync)
				r.Delete("/", integrations.taskInboxDelete)
			})
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
// HTML files are sent with no-cache headers so browsers always fetch the latest entry point.
// Hashed JS/CSS assets get long-lived caching from the browser's default behaviour.
func spaHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			w.Header().Set("Cache-Control", "no-store")
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		// Prevent caching of HTML files (SPA entry points change on every deploy)
		if strings.HasSuffix(r.URL.Path, ".html") || r.URL.Path == "/" {
			w.Header().Set("Cache-Control", "no-store")
		}
		fs.ServeHTTP(w, r)
	})
}
