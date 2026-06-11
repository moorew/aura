package api

import (
	"encoding/json"
	"net/http"

	"github.com/clevercode/sempa/internal/db"
	"github.com/clevercode/sempa/internal/notify"
)

type notificationHandler struct {
	configs  *db.IntegrationConfigStore
	pushSubs *db.PushSubStore
	vapidPub string
}

// getSettings returns the current notification settings, falling back to
// defaults when nothing has been saved yet.
func (h *notificationHandler) getSettings(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusOK, notify.LoadSettings(r.Context(), h.configs))
}

// putSettings persists the notification settings document.
func (h *notificationHandler) putSettings(w http.ResponseWriter, r *http.Request) {
	var s notify.Settings
	if err := decode(r, &s); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	raw, err := json.Marshal(s)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to encode settings")
		return
	}
	if _, err := h.configs.Upsert(r.Context(), newID(), "notifications", string(raw)); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save settings")
		return
	}
	respond(w, http.StatusOK, s)
}

// vapidKey hands the browser the application server public key it needs to call
// pushManager.subscribe().
func (h *notificationHandler) vapidKey(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusOK, map[string]string{"key": h.vapidPub})
}

type subscribeRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
	Platform string `json:"platform"`
}

// subscribe stores a browser Web Push subscription (PushSubscription.toJSON()).
func (h *notificationHandler) subscribe(w http.ResponseWriter, r *http.Request) {
	var req subscribeRequest
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Endpoint == "" || req.Keys.P256dh == "" || req.Keys.Auth == "" {
		respondError(w, http.StatusBadRequest, "endpoint and keys are required")
		return
	}
	sub, err := h.pushSubs.Upsert(newID(), req.Endpoint, req.Keys.P256dh, req.Keys.Auth, req.Platform)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save subscription")
		return
	}
	respond(w, http.StatusOK, sub)
}

// unsubscribe removes a Web Push subscription by endpoint.
func (h *notificationHandler) unsubscribe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := decode(r, &req); err != nil || req.Endpoint == "" {
		respondError(w, http.StatusBadRequest, "endpoint is required")
		return
	}
	if err := h.pushSubs.DeleteByEndpoint(req.Endpoint); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to remove subscription")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// testWebhook validates the generic webhook configuration as entered in the
// settings form (tests the posted config, not necessarily the saved one).
func (h *notificationHandler) testWebhook(w http.ResponseWriter, r *http.Request) {
	var cfg notify.WebhookConfig
	if err := decode(r, &cfg); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := notify.TestWebhook(cfg); err != nil {
		respondError(w, http.StatusBadGateway, "webhook test failed: "+err.Error())
		return
	}
	respond(w, http.StatusOK, map[string]bool{"ok": true})
}
