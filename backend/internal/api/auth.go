package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/clevercode/sempa/internal/config"
)

const sessionCookieName = "sempa_session"

// ── Session store ─────────────────────────────────────────────────────────────

type sessionEntry struct {
	Email   string
	Expires time.Time
}

type sessionStore struct {
	mu      sync.Mutex
	entries map[string]sessionEntry
}

func newSessionStore() *sessionStore {
	s := &sessionStore{entries: make(map[string]sessionEntry)}
	go s.reap()
	return s
}

func (s *sessionStore) create(ttl time.Duration, email string) string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	s.mu.Lock()
	s.entries[id] = sessionEntry{Email: email, Expires: time.Now().Add(ttl)}
	s.mu.Unlock()
	return id
}

func (s *sessionStore) get(id string) (sessionEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[id]
	if !ok || time.Now().After(e.Expires) {
		return sessionEntry{}, false
	}
	return e, true
}

func (s *sessionStore) delete(id string) {
	s.mu.Lock()
	delete(s.entries, id)
	s.mu.Unlock()
}

func (s *sessionStore) reap() {
	for range time.Tick(10 * time.Minute) {
		now := time.Now()
		s.mu.Lock()
		for id, e := range s.entries {
			if now.After(e.Expires) {
				delete(s.entries, id)
			}
		}
		s.mu.Unlock()
	}
}

// ── OAuth state store (anti-CSRF) ─────────────────────────────────────────────

type stateEntry struct {
	Redirect string
	Expires  time.Time
}

type stateStore struct {
	mu      sync.Mutex
	entries map[string]stateEntry
}

func newStateStore() *stateStore {
	s := &stateStore{entries: make(map[string]stateEntry)}
	go s.reap()
	return s
}

func (s *stateStore) create(redirect string) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	s.mu.Lock()
	s.entries[id] = stateEntry{Redirect: redirect, Expires: time.Now().Add(15 * time.Minute)}
	s.mu.Unlock()
	return id
}

func (s *stateStore) pop(id string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[id]
	if !ok || time.Now().After(e.Expires) {
		return "", false
	}
	delete(s.entries, id)
	return e.Redirect, true
}

func (s *stateStore) reap() {
	for range time.Tick(5 * time.Minute) {
		now := time.Now()
		s.mu.Lock()
		for id, e := range s.entries {
			if now.After(e.Expires) {
				delete(s.entries, id)
			}
		}
		s.mu.Unlock()
	}
}

// ── Auth handler ──────────────────────────────────────────────────────────────

type authHandler struct {
	cfg      config.Config
	sessions *sessionStore
	states   *stateStore
}

func newAuthHandler(cfg config.Config) *authHandler {
	return &authHandler{
		cfg:      cfg,
		sessions: newSessionStore(),
		states:   newStateStore(),
	}
}

func (h *authHandler) passwordEnabled() bool { return h.cfg.AuthPassword != "" }
func (h *authHandler) googleEnabled() bool   { return h.cfg.GmailClientID != "" && h.cfg.GmailClientSecret != "" }

// authEnabled returns true when any auth mechanism is configured.
func (h *authHandler) authEnabled() bool { return h.passwordEnabled() || h.googleEnabled() }

func (h *authHandler) emailAllowed(email string) bool {
	if len(h.cfg.AllowedEmails) == 0 {
		return true
	}
	email = strings.ToLower(strings.TrimSpace(email))
	for _, a := range h.cfg.AllowedEmails {
		if a == email {
			return true
		}
	}
	return false
}

func (h *authHandler) googleCallbackURL() string {
	return h.cfg.AppURL + "/api/v1/auth/google/callback"
}

func (h *authHandler) setSessionCookie(w http.ResponseWriter, id string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    id,
		HttpOnly: true,
		Secure:   h.cfg.Env == "production",
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
	})
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	if !h.authEnabled() {
		respond(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if !h.passwordEnabled() {
		respondError(w, http.StatusBadRequest, "password auth is not configured; use Google Sign-In")
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

	id := h.sessions.create(30*24*time.Hour, h.cfg.AuthUsername)
	h.setSessionCookie(w, id)
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
		respond(w, http.StatusOK, map[string]any{
			"authenticated": true,
			"auth_enabled":  false,
			"google_enabled": h.googleEnabled(),
		})
		return
	}
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		respond(w, http.StatusOK, map[string]any{
			"authenticated":  false,
			"auth_enabled":   true,
			"google_enabled": h.googleEnabled(),
		})
		return
	}
	entry, ok := h.sessions.get(c.Value)
	if !ok {
		respond(w, http.StatusOK, map[string]any{
			"authenticated":  false,
			"auth_enabled":   true,
			"google_enabled": h.googleEnabled(),
		})
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"authenticated":  true,
		"auth_enabled":   true,
		"google_enabled": h.googleEnabled(),
		"email":          entry.Email,
	})
}

func (h *authHandler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.authEnabled() {
			next.ServeHTTP(w, r)
			return
		}
		c, err := r.Cookie(sessionCookieName)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		if _, ok := h.sessions.get(c.Value); !ok {
			respondError(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ── Google OAuth ──────────────────────────────────────────────────────────────

func (h *authHandler) googleAuth(w http.ResponseWriter, r *http.Request) {
	if !h.googleEnabled() {
		http.Error(w, "Google sign-in is not configured", http.StatusServiceUnavailable)
		return
	}
	redirect := r.URL.Query().Get("redirect")
	if redirect == "" {
		redirect = "/"
	}
	state := h.states.create(redirect)

	q := url.Values{
		"client_id":     {h.cfg.GmailClientID},
		"redirect_uri":  {h.googleCallbackURL()},
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"state":         {state},
		"access_type":   {"online"},
		"prompt":        {"select_account"},
	}
	http.Redirect(w, r, "https://accounts.google.com/o/oauth2/v2/auth?"+q.Encode(), http.StatusFound)
}

func (h *authHandler) googleCallback(w http.ResponseWriter, r *http.Request) {
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		http.Redirect(w, r, "/login?error="+url.QueryEscape(errParam), http.StatusFound)
		return
	}

	state := r.URL.Query().Get("state")
	redirect, ok := h.states.pop(state)
	if !ok {
		http.Error(w, "invalid or expired state — please try signing in again", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code from Google", http.StatusBadRequest)
		return
	}

	accessToken, err := h.exchangeCode(r, code)
	if err != nil {
		http.Error(w, fmt.Sprintf("token exchange failed: %v", err), http.StatusBadGateway)
		return
	}

	email, err := getGoogleEmail(r.Context(), accessToken)
	if err != nil {
		http.Error(w, fmt.Sprintf("userinfo failed: %v", err), http.StatusBadGateway)
		return
	}

	if !h.emailAllowed(email) {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("not_allowed"), http.StatusFound)
		return
	}

	id := h.sessions.create(30*24*time.Hour, email)
	h.setSessionCookie(w, id)
	http.Redirect(w, r, redirect, http.StatusFound)
}

func (h *authHandler) exchangeCode(r *http.Request, code string) (string, error) {
	body := url.Values{
		"code":          {code},
		"client_id":     {h.cfg.GmailClientID},
		"client_secret": {h.cfg.GmailClientSecret},
		"redirect_uri":  {h.googleCallbackURL()},
		"grant_type":    {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost,
		"https://oauth2.googleapis.com/token", bytes.NewBufferString(body.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, raw)
	}

	var tok struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil {
		return "", err
	}
	if tok.Error != "" {
		return "", fmt.Errorf("google: %s", tok.Error)
	}
	return tok.AccessToken, nil
}

func getGoogleEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("userinfo HTTP %d: %s", resp.StatusCode, body)
	}

	var info struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}
	if info.Email == "" {
		return "", fmt.Errorf("no email in Google userinfo response")
	}
	return info.Email, nil
}
