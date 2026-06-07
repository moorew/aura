package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/backup"
	"github.com/clevercode/sempa/internal/config"
	"github.com/clevercode/sempa/internal/db"
	"github.com/clevercode/sempa/internal/integrations/gmail"
)

// backupDriveType is the integration_configs key for the backup Drive token.
const backupDriveType = backup.DriveConfigType

func driveRedirectURI(cfg config.Config) string {
	return cfg.AppURL + "/api/v1/backup/drive/callback"
}

// driveAuth starts the Google consent flow for the drive.file scope.
func (h *backupHandler) driveAuth(w http.ResponseWriter, r *http.Request) {
	if h.cfg.GmailClientID == "" {
		respondError(w, http.StatusServiceUnavailable, "Google OAuth is not configured on this server")
		return
	}
	state := gmail.GenerateState()
	url := gmail.AuthURLForScopes(h.cfg.GmailClientID, driveRedirectURI(h.cfg), state, gmail.ScopeDriveFile)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// driveCallback exchanges the code and stores the Drive token.
func (h *backupHandler) driveCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if !gmail.ConsumeState(state) {
		respondError(w, http.StatusBadRequest, "invalid or expired OAuth state")
		return
	}
	stored, err := gmail.ExchangeCode(r.Context(), h.cfg.GmailClientID, h.cfg.GmailClientSecret, driveRedirectURI(h.cfg), code)
	if err != nil {
		respondError(w, http.StatusBadGateway, "token exchange failed: "+err.Error())
		return
	}
	if email, err := gmail.FetchEmail(r.Context(), stored.AccessToken); err == nil {
		stored.Email = email
	}
	cfgJSON, _ := json.Marshal(stored)
	if _, err := h.configs.Upsert(r.Context(), uuid.New().String(), backupDriveType, string(cfgJSON)); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	http.Redirect(w, r, h.cfg.FrontendURL+"/settings/backup?drive=connected", http.StatusTemporaryRedirect)
}

func (h *backupHandler) driveStatus(w http.ResponseWriter, r *http.Request) {
	c, err := h.configs.Get(r.Context(), backupDriveType)
	if errors.Is(err, db.ErrNotFound) {
		respond(w, http.StatusOK, map[string]any{"connected": false})
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var tok gmail.StoredToken
	_ = json.Unmarshal([]byte(c.Config), &tok)
	respond(w, http.StatusOK, map[string]any{"connected": true, "email": tok.Email})
}

func (h *backupHandler) driveDisconnect(w http.ResponseWriter, r *http.Request) {
	if err := h.configs.Delete(r.Context(), backupDriveType); err != nil && !errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusNoContent, nil)
}

// driveTokenFunc resolves a Google Drive access token from the stored token.
func driveTokenFunc(configs *db.IntegrationConfigStore, cfg config.Config) backup.DriveTokenFunc {
	return backup.DriveTokenResolver(configs, cfg.GmailClientID, cfg.GmailClientSecret)
}
