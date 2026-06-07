package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

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
		writeOAuthResultPage(w, h.cfg.FrontendURL, false, "This sign-in link expired. Please try connecting again.")
		return
	}
	stored, err := gmail.ExchangeCode(r.Context(), h.cfg.GmailClientID, h.cfg.GmailClientSecret, driveRedirectURI(h.cfg), code)
	if err != nil {
		writeOAuthResultPage(w, h.cfg.FrontendURL, false, "Token exchange failed: "+err.Error())
		return
	}
	if email, err := gmail.FetchEmail(r.Context(), stored.AccessToken); err == nil {
		stored.Email = email
	}
	cfgJSON, _ := json.Marshal(stored)
	if _, err := h.configs.Upsert(r.Context(), uuid.New().String(), backupDriveType, string(cfgJSON)); err != nil {
		writeOAuthResultPage(w, h.cfg.FrontendURL, false, "Could not save the connection. Please try again.")
		return
	}
	writeOAuthResultPage(w, h.cfg.FrontendURL, true, "")
}

// writeOAuthResultPage renders a small self-contained page after the Drive OAuth
// redirect. It works for every client: a web popup is notified via postMessage
// and auto-closes; everything else shows a clear "you can return to Sempa"
// message instead of dumping the user into the web app's login screen.
func writeOAuthResultPage(w http.ResponseWriter, frontendURL string, ok bool, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if ok {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
	title := "Google Drive connected"
	icon := "✓"
	color := "#16a34a"
	body := "You can close this window and return to Sempa."
	if !ok {
		title = "Couldn’t connect Google Drive"
		icon = "✕"
		color = "#dc2626"
		body = htmlEscape(errMsg)
	}
	backURL := htmlEscape(frontendURL) + "/settings/backup?drive=" + map[bool]string{true: "connected", false: "error"}[ok]
	msg := "sempa-drive-" + map[bool]string{true: "connected", false: "error"}[ok]
	fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<style>
 body{font-family:-apple-system,Segoe UI,Roboto,sans-serif;background:#faf7f2;color:#2b2620;
 display:flex;min-height:100vh;align-items:center;justify-content:center;margin:0;padding:24px}
 .card{max-width:380px;text-align:center;background:#fff;border-radius:18px;padding:36px 28px;
 box-shadow:0 8px 30px rgba(0,0,0,.08)}
 .icon{width:56px;height:56px;border-radius:50%%;background:%s;color:#fff;font-size:28px;line-height:56px;margin:0 auto 16px}
 h1{font-size:18px;margin:0 0 8px} p{font-size:14px;color:#6b6358;margin:0 0 20px;line-height:1.5}
 a{display:inline-block;background:#b3592e;color:#fff;text-decoration:none;padding:10px 20px;border-radius:10px;font-size:14px;font-weight:600}
</style></head><body>
<div class="card">
 <div class="icon">%s</div>
 <h1>%s</h1>
 <p>%s</p>
 <a href="%s">Return to Sempa</a>
</div>
<script>
 try { if (window.opener) { window.opener.postMessage('%s','*'); setTimeout(function(){window.close();}, 400); } } catch (e) {}
</script>
</body></html>`, htmlEscape(title), color, icon, htmlEscape(title), body, backURL, msg)
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
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
