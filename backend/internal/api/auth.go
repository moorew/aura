package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/clevercode/aura/internal/config"
)

const sessionCookieName = "aura_session"

type sessionStore struct {
	mu       sync.Mutex
	sessions map[string]time.Time
}

func newSessionStore() *sessionStore {
	s := &sessionStore{sessions: make(map[string]time.Time)}
	go s.reap()
	return s
}

func (s *sessionStore) create(ttl time.Duration) string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	s.mu.Lock()
	s.sessions[id] = time.Now().Add(ttl)
	s.mu.Unlock()
	return id
}

func (s *sessionStore) valid(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.sessions[id]
	return ok && time.Now().Before(exp)
}

func (s *sessionStore) delete(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

func (s *sessionStore) reap() {
	for range time.Tick(10 * time.Minute) {
		now := time.Now()
		s.mu.Lock()
		for id, exp := range s.sessions {
			if now.After(exp) {
				delete(s.sessions, id)
			}
		}
		s.mu.Unlock()
	}
}

type authHandler struct {
	cfg      config.Config
	sessions *sessionStore
}

func newAuthHandler(cfg config.Config) *authHandler {
	return &authHandler{cfg: cfg, sessions: newSessionStore()}
}

// authEnabled returns true when a password is configured.
func (h *authHandler) authEnabled() bool { return h.cfg.AuthPassword != "" }

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	if !h.authEnabled() {
		respond(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	userMatch := subtle.ConstantTimeCompare([]byte(req.Username), []byte(h.cfg.AuthUsername)) == 1
	passMatch := subtle.ConstantTimeCompare([]byte(req.Password), []byte(h.cfg.AuthPassword)) == 1
	if !userMatch || !passMatch {
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	id := h.sessions.create(30 * 24 * time.Hour)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    id,
		HttpOnly: true,
		Secure:   h.cfg.Env == "production",
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
	})
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *authHandler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil {
		h.sessions.delete(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:   sessionCookieName,
		Value:  "",
		MaxAge: -1,
		Path:   "/",
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandler) me(w http.ResponseWriter, r *http.Request) {
	if !h.authEnabled() {
		respond(w, http.StatusOK, map[string]any{"authenticated": true, "auth_enabled": false})
		return
	}
	c, err := r.Cookie(sessionCookieName)
	if err != nil || !h.sessions.valid(c.Value) {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	respond(w, http.StatusOK, map[string]any{"authenticated": true, "auth_enabled": true, "username": h.cfg.AuthUsername})
}

// requireAuth is middleware that gates all API routes when auth is enabled.
func (h *authHandler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.authEnabled() {
			next.ServeHTTP(w, r)
			return
		}
		c, err := r.Cookie(sessionCookieName)
		if err != nil || !h.sessions.valid(c.Value) {
			respondError(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		next.ServeHTTP(w, r)
	})
}
