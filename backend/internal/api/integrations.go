package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/config"
	"github.com/clevercode/sempa/internal/db"
	"github.com/clevercode/sempa/internal/integrations/emailrecv"
	"github.com/clevercode/sempa/internal/integrations/fastmail"
	"github.com/clevercode/sempa/internal/integrations/gmail"
	"github.com/clevercode/sempa/internal/integrations/jira"
)

type integrationHandler struct {
	configs *db.IntegrationConfigStore
	tasks   *db.TaskStore
	cfg     config.Config
}

// ── Jira ─────────────────────────────────────────────────────────────────────

func (h *integrationHandler) jiraGet(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configs.Get(r.Context(), "jira")
	if errors.Is(err, db.ErrNotFound) {
		respond(w, http.StatusOK, map[string]any{"connected": false})
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var raw jira.Config
	if err := json.Unmarshal([]byte(cfg.Config), &raw); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}
	raw.APIToken = "" // never send the token back to the client

	respond(w, http.StatusOK, map[string]any{
		"connected":      true,
		"enabled":        cfg.Enabled,
		"last_synced_at": cfg.LastSyncedAt,
		"config":         raw,
	})
}

func (h *integrationHandler) jiraPut(w http.ResponseWriter, r *http.Request) {
	var body jira.Config
	if err := decode(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Host == "" || body.Email == "" || body.APIToken == "" {
		respondError(w, http.StatusBadRequest, "host, email, and api_token are required")
		return
	}

	// If updating and existing record has the token, preserve existing token if
	// the client sent an empty string (redacted round-trip).
	if body.APIToken == "" {
		existing, err := h.configs.Get(r.Context(), "jira")
		if err == nil {
			var prev jira.Config
			if json.Unmarshal([]byte(existing.Config), &prev) == nil {
				body.APIToken = prev.APIToken
			}
		}
	}

	configJSON, err := json.Marshal(body)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	cfg, err := h.configs.Upsert(r.Context(), uuid.New().String(), "jira", string(configJSON))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusOK, cfg)
}

func (h *integrationHandler) jiraTest(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configs.Get(r.Context(), "jira")
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusBadRequest, "jira not configured")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var jiraCfg jira.Config
	if err := json.Unmarshal([]byte(cfg.Config), &jiraCfg); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}

	client := jira.NewClient(jiraCfg)
	if err := client.TestConnection(r.Context()); err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *integrationHandler) jiraSync(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configs.Get(r.Context(), "jira")
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusBadRequest, "jira not configured")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var jiraCfg jira.Config
	if err := json.Unmarshal([]byte(cfg.Config), &jiraCfg); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}

	result, err := jira.Sync(r.Context(), jiraCfg, h.tasks)
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}

	_ = h.configs.TouchSyncTime(r.Context(), "jira")
	respond(w, http.StatusOK, result)
}

func (h *integrationHandler) jiraDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.configs.Delete(r.Context(), "jira"); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			respondError(w, http.StatusNotFound, "jira not configured")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Gmail ─────────────────────────────────────────────────────────────────────

func (h *integrationHandler) gmailGet(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configs.Get(r.Context(), "gmail")
	if errors.Is(err, db.ErrNotFound) {
		respond(w, http.StatusOK, map[string]any{"connected": false})
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var stored gmail.StoredToken
	if err := json.Unmarshal([]byte(cfg.Config), &stored); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}

	respond(w, http.StatusOK, map[string]any{
		"connected":      true,
		"enabled":        cfg.Enabled,
		"email":          stored.Email,
		"labels":         stored.Labels,
		"last_synced_at": cfg.LastSyncedAt,
	})
}

func (h *integrationHandler) gmailAuth(w http.ResponseWriter, r *http.Request) {
	if h.cfg.GmailClientID == "" {
		respondError(w, http.StatusServiceUnavailable, "Gmail OAuth not configured on this server")
		return
	}
	withCalendar := r.URL.Query().Get("calendar") == "1"
	state := gmail.GenerateState()
	redirectURI := h.cfg.AppURL + "/api/v1/integrations/gmail/callback"
	authURL := gmail.AuthURL(h.cfg.GmailClientID, redirectURI, state, withCalendar)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (h *integrationHandler) gmailCallback(w http.ResponseWriter, r *http.Request) {
	code  := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if !gmail.ConsumeState(state) {
		respondError(w, http.StatusBadRequest, "invalid or expired OAuth state")
		return
	}

	redirectURI := h.cfg.AppURL + "/api/v1/integrations/gmail/callback"
	stored, err := gmail.ExchangeCode(r.Context(), h.cfg.GmailClientID, h.cfg.GmailClientSecret, redirectURI, code)
	if err != nil {
		respondError(w, http.StatusBadGateway, "token exchange failed: "+err.Error())
		return
	}

	email, err := gmail.FetchEmail(r.Context(), stored.AccessToken)
	if err == nil {
		stored.Email = email
	}

	configJSON, err := json.Marshal(stored)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := h.configs.Upsert(r.Context(), uuid.New().String(), "gmail", string(configJSON)); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Redirect back to the frontend settings page
	http.Redirect(w, r, h.cfg.FrontendURL+"/settings/integrations/gmail?connected=1", http.StatusTemporaryRedirect)
}

func (h *integrationHandler) gmailUpdateLabels(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configs.Get(r.Context(), "gmail")
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusBadRequest, "gmail not connected")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var body struct {
		Labels []string `json:"labels"`
	}
	if err := decode(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	var stored gmail.StoredToken
	if err := json.Unmarshal([]byte(cfg.Config), &stored); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}
	stored.Labels = body.Labels

	configJSON, err := json.Marshal(stored)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, err := h.configs.UpdateConfig(r.Context(), "gmail", string(configJSON))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusOK, updated)
}

func (h *integrationHandler) gmailSync(w http.ResponseWriter, r *http.Request) {
	if h.cfg.GmailClientID == "" {
		respondError(w, http.StatusServiceUnavailable, "Gmail OAuth not configured on this server")
		return
	}

	cfg, err := h.configs.Get(r.Context(), "gmail")
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusBadRequest, "gmail not connected")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var stored gmail.StoredToken
	if err := json.Unmarshal([]byte(cfg.Config), &stored); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}

	result, err := gmail.Sync(r.Context(), h.cfg.GmailClientID, h.cfg.GmailClientSecret, &stored, h.tasks)
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}

	// Persist refreshed token if it changed
	configJSON, _ := json.Marshal(stored)
	_, _ = h.configs.UpdateConfig(r.Context(), "gmail", string(configJSON))
	_ = h.configs.TouchSyncTime(r.Context(), "gmail")

	respond(w, http.StatusOK, result)
}

func (h *integrationHandler) gmailDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.configs.Delete(r.Context(), "gmail"); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			respondError(w, http.StatusNotFound, "gmail not connected")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Calendar (shares Gmail OAuth token) ──────────────────────────────────────

func (h *integrationHandler) calendarGet(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configs.Get(r.Context(), "gmail")
	if errors.Is(err, db.ErrNotFound) {
		respond(w, http.StatusOK, map[string]any{"connected": false})
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var stored gmail.StoredToken
	if err := json.Unmarshal([]byte(cfg.Config), &stored); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"connected":       stored.CalendarEnabled,
		"email":           stored.Email,
		"calendar_ids":    stored.CalendarIDs,
		"last_synced_at":  cfg.LastSyncedAt,
	})
}

func (h *integrationHandler) calendarToggle(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configs.Get(r.Context(), "gmail")
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusBadRequest, "gmail not connected — connect Gmail first to enable calendar")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var body struct {
		Enabled     bool     `json:"enabled"`
		CalendarIDs []string `json:"calendar_ids"`
	}
	if err := decode(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	var stored gmail.StoredToken
	if err := json.Unmarshal([]byte(cfg.Config), &stored); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}
	stored.CalendarEnabled = body.Enabled
	if body.CalendarIDs != nil {
		stored.CalendarIDs = body.CalendarIDs
	}

	configJSON, _ := json.Marshal(stored)
	if _, err := h.configs.UpdateConfig(r.Context(), "gmail", string(configJSON)); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusOK, map[string]any{"enabled": stored.CalendarEnabled})
}

func (h *integrationHandler) calendarSync(w http.ResponseWriter, r *http.Request) {
	if h.cfg.GmailClientID == "" {
		respondError(w, http.StatusServiceUnavailable, "Gmail OAuth not configured")
		return
	}
	cfg, err := h.configs.Get(r.Context(), "gmail")
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusBadRequest, "gmail not connected")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var stored gmail.StoredToken
	if err := json.Unmarshal([]byte(cfg.Config), &stored); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}

	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	result, err := gmail.SyncCalendar(r.Context(), h.cfg.GmailClientID, h.cfg.GmailClientSecret, &stored, h.tasks, date)
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}

	configJSON, _ := json.Marshal(stored)
	_, _ = h.configs.UpdateConfig(r.Context(), "gmail", string(configJSON))

	respond(w, http.StatusOK, result)
}

// ── Fastmail ──────────────────────────────────────────────────────────────────

func (h *integrationHandler) fastmailGet(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configs.Get(r.Context(), "fastmail")
	if errors.Is(err, db.ErrNotFound) {
		respond(w, http.StatusOK, map[string]any{"connected": false})
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var raw fastmail.Config
	if err := json.Unmarshal([]byte(cfg.Config), &raw); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"connected":      true,
		"enabled":        cfg.Enabled,
		"email":          raw.Email,
		"inbox_address":  raw.InboxAddress,
		"last_synced_at": cfg.LastSyncedAt,
	})
}

func (h *integrationHandler) fastmailPut(w http.ResponseWriter, r *http.Request) {
	var body fastmail.Config
	if err := decode(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Email == "" || body.AppPassword == "" {
		respondError(w, http.StatusBadRequest, "email and app_password are required")
		return
	}

	if err := fastmail.TestConnectionIMAP(body.Email, body.AppPassword); err != nil {
		respondError(w, http.StatusBadGateway, "connection failed: "+err.Error())
		return
	}

	configJSON, _ := json.Marshal(body)
	cfg, err := h.configs.Upsert(r.Context(), uuid.New().String(), "fastmail", string(configJSON))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusOK, cfg)
}

func (h *integrationHandler) fastmailSync(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configs.Get(r.Context(), "fastmail")
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusBadRequest, "fastmail not connected")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var fmCfg fastmail.Config
	if err := json.Unmarshal([]byte(cfg.Config), &fmCfg); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}

	result, err := fastmail.Sync(r.Context(), fmCfg, h.tasks)
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}

	_ = h.configs.TouchSyncTime(r.Context(), "fastmail")
	respond(w, http.StatusOK, result)
}

func (h *integrationHandler) fastmailPatch(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configs.Get(r.Context(), "fastmail")
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusBadRequest, "fastmail not connected")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var stored fastmail.Config
	if err := json.Unmarshal([]byte(cfg.Config), &stored); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}
	var patch struct {
		InboxAddress *string `json:"inbox_address"`
	}
	if err := decode(r, &patch); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if patch.InboxAddress != nil {
		stored.InboxAddress = *patch.InboxAddress
	}
	configJSON, _ := json.Marshal(stored)
	if _, err := h.configs.UpdateConfig(r.Context(), "fastmail", string(configJSON)); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusOK, map[string]any{"inbox_address": stored.InboxAddress})
}

func (h *integrationHandler) fastmailInboxSync(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configs.Get(r.Context(), "fastmail")
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusBadRequest, "fastmail not connected")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var fmCfg fastmail.Config
	if err := json.Unmarshal([]byte(cfg.Config), &fmCfg); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}
	if fmCfg.InboxAddress == "" {
		respondError(w, http.StatusBadRequest, "no inbox address configured")
		return
	}
	result, err := fastmail.SyncInbox(r.Context(), fmCfg, h.tasks)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = h.configs.TouchSyncTime(r.Context(), "fastmail")
	respond(w, http.StatusOK, result)
}

func (h *integrationHandler) fastmailDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.configs.Delete(r.Context(), "fastmail"); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			respondError(w, http.StatusNotFound, "fastmail not connected")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *integrationHandler) emailForwardGet(w http.ResponseWriter, r *http.Request) {
	// SMTP address (Tailscale-internal, requires port spec)
	smtpEnabled := h.cfg.SMTPPort != ""
	smtpAddress := ""
	if smtpEnabled {
		host := h.cfg.AppURL
		for _, prefix := range []string{"https://", "http://"} {
			host = strings.TrimPrefix(host, prefix)
		}
		if idx := strings.Index(host, "/"); idx >= 0 {
			host = host[:idx]
		}
		smtpAddress = "tasks@" + host + ":" + h.cfg.SMTPPort
	}

	// Webhook (Cloudflare Email Routing)
	webhookEnabled := h.cfg.EmailForwardToken != ""
	webhookURL := ""
	if webhookEnabled {
		webhookURL = h.cfg.AppURL + "/api/v1/tasks/from-email"
	}

	respond(w, http.StatusOK, map[string]any{
		"smtp_enabled":    smtpEnabled,
		"smtp_address":    smtpAddress,
		"smtp_port":       h.cfg.SMTPPort,
		"webhook_enabled": webhookEnabled,
		"webhook_url":     webhookURL,
	})
}

// fromEmail accepts a raw RFC 5322 email via POST (used by Cloudflare Email Workers).
// Protected by Bearer token in Authorization header.
func (h *integrationHandler) fromEmail(w http.ResponseWriter, r *http.Request) {
	if h.cfg.EmailForwardToken == "" {
		respondError(w, http.StatusServiceUnavailable, "email forwarding not configured")
		return
	}
	auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if auth != h.cfg.EmailForwardToken {
		respondError(w, http.StatusUnauthorized, "invalid token")
		return
	}
	if err := emailrecv.CreateFromReader(r.Context(), r.Body, h.tasks); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Task inbox (standalone email forwarding) ──────────────────────────────

func (h *integrationHandler) taskInboxGet(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configs.Get(r.Context(), "task_inbox")
	if errors.Is(err, db.ErrNotFound) {
		respond(w, http.StatusOK, map[string]any{"connected": false})
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var raw fastmail.InboxConfig
	if err := json.Unmarshal([]byte(cfg.Config), &raw); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}
	if raw.AllowedSenders == nil {
		raw.AllowedSenders = []string{}
	}
	respond(w, http.StatusOK, map[string]any{
		"connected":       true,
		"email":           raw.Email,
		"inbox_address":   raw.InboxAddress,
		"allowed_senders": raw.AllowedSenders,
		"last_synced_at":  cfg.LastSyncedAt,
	})
}

func (h *integrationHandler) taskInboxPut(w http.ResponseWriter, r *http.Request) {
	var body fastmail.InboxConfig
	if err := decode(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Email == "" || body.AppPassword == "" || body.InboxAddress == "" {
		respondError(w, http.StatusBadRequest, "email, app_password, and inbox_address are required")
		return
	}
	if err := fastmail.TestConnectionIMAP(body.Email, body.AppPassword); err != nil {
		respondError(w, http.StatusBadGateway, "connection failed: "+err.Error())
		return
	}
	if body.AllowedSenders == nil {
		body.AllowedSenders = []string{}
	}
	configJSON, _ := json.Marshal(body)
	if _, err := h.configs.Upsert(r.Context(), uuid.New().String(), "task_inbox", string(configJSON)); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"connected":       true,
		"email":           body.Email,
		"inbox_address":   body.InboxAddress,
		"allowed_senders": body.AllowedSenders,
	})
}

func (h *integrationHandler) taskInboxPatchSenders(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configs.Get(r.Context(), "task_inbox")
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusBadRequest, "task inbox not connected")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var stored fastmail.InboxConfig
	if err := json.Unmarshal([]byte(cfg.Config), &stored); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}
	var patch struct {
		AllowedSenders []string `json:"allowed_senders"`
	}
	if err := decode(r, &patch); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	stored.AllowedSenders = patch.AllowedSenders
	if stored.AllowedSenders == nil {
		stored.AllowedSenders = []string{}
	}
	configJSON, _ := json.Marshal(stored)
	if _, err := h.configs.UpdateConfig(r.Context(), "task_inbox", string(configJSON)); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusOK, map[string]any{"allowed_senders": stored.AllowedSenders})
}

func (h *integrationHandler) taskInboxSync(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configs.Get(r.Context(), "task_inbox")
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusBadRequest, "task inbox not connected")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var inboxCfg fastmail.InboxConfig
	if err := json.Unmarshal([]byte(cfg.Config), &inboxCfg); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}
	result, err := fastmail.SyncTaskInbox(r.Context(), inboxCfg, h.tasks)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = h.configs.TouchSyncTime(r.Context(), "task_inbox")
	respond(w, http.StatusOK, result)
}

func (h *integrationHandler) taskInboxDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.configs.Delete(r.Context(), "task_inbox"); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			respondError(w, http.StatusNotFound, "task inbox not connected")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Fastmail inbox panel ──────────────────────────────────────────────────

func (h *integrationHandler) fastmailEmails(w http.ResponseWriter, r *http.Request) {
	fmCfg, err := h.getFastmailConfig(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	emails, err := fastmail.GetIMAPInboxEmails(fmCfg.Email, fmCfg.AppPassword, 50)
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	type emailRow struct {
		ID         string                 `json:"id"`
		Subject    string                 `json:"subject"`
		From       []fastmail.EmailAddress `json:"from"`
		ReceivedAt string                 `json:"received_at"`
		Preview    string                 `json:"preview"`
		IsUnread   bool                   `json:"is_unread"`
	}
	rows := make([]emailRow, len(emails))
	for i, e := range emails {
		rows[i] = emailRow{
			ID: e.ID, Subject: e.Subject, From: e.From,
			ReceivedAt: e.ReceivedAt, IsUnread: e.IsUnread(),
		}
	}
	respond(w, http.StatusOK, rows)
}

func (h *integrationHandler) fastmailEmailToTask(w http.ResponseWriter, r *http.Request) {
	fmCfg, err := h.getFastmailConfig(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	uid, err := parseUID(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid email id")
		return
	}

	var req struct{ Subject string `json:"subject"` }
	_ = decode(r, &req)
	subject := req.Subject
	if subject == "" {
		subject = "(no subject)"
	}

	bodyText, _ := fastmail.GetIMAPEmailBody(fmCfg.Email, fmCfg.AppPassword, uid)

	today := time.Now().Format("2006-01-02")
	ws := mondayOfDate(today)
	source := "fastmail"
	sourceID := "panel_" + chi.URLParam(r, "id")
	sourceURL := "https://app.fastmail.com/mail/"

	var desc *string
	if bodyText != "" {
		d := bodyText
		if len(d) > 4000 {
			d = d[:4000] + "…"
		}
		desc = &d
	}

	task, createErr := h.tasks.Create(r.Context(), db.CreateTaskParams{
		ID: newID(), Title: subject, Description: desc,
		Status: "planned", PlannedDate: &today, WeekStart: &ws,
		Position: float64(time.Now().UnixMilli()),
		Source: &source, SourceID: &sourceID, SourceURL: &sourceURL,
		Tags: []string{},
	})
	if createErr != nil {
		respondError(w, http.StatusInternalServerError, createErr.Error())
		return
	}
	_ = fastmail.ArchiveIMAPEmail(fmCfg.Email, fmCfg.AppPassword, uid)
	respond(w, http.StatusOK, task)
}

func (h *integrationHandler) fastmailArchiveEmail(w http.ResponseWriter, r *http.Request) {
	fmCfg, err := h.getFastmailConfig(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	uid, err := parseUID(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid email id")
		return
	}
	if err := fastmail.ArchiveIMAPEmail(fmCfg.Email, fmCfg.AppPassword, uid); err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getFastmailConfig loads the Fastmail integration config or returns an error.
func (h *integrationHandler) getFastmailConfig(r *http.Request) (fastmail.Config, error) {
	cfg, err := h.configs.Get(r.Context(), "fastmail")
	if errors.Is(err, db.ErrNotFound) {
		return fastmail.Config{}, fmt.Errorf("fastmail not connected")
	}
	if err != nil {
		return fastmail.Config{}, err
	}
	var fmCfg fastmail.Config
	if err := json.Unmarshal([]byte(cfg.Config), &fmCfg); err != nil {
		return fastmail.Config{}, fmt.Errorf("malformed config")
	}
	return fmCfg, nil
}

func parseUID(s string) (uint32, error) {
	var uid uint64
	_, err := fmt.Sscan(s, &uid)
	return uint32(uid), err
}
