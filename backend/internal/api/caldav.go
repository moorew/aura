package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/db"
	"github.com/clevercode/sempa/internal/integrations/caldav"
)

// caldavConfig is the JSON stored in the "caldav" integration config row. It
// records the chosen calendar; credentials are reused from the "fastmail" row.
type caldavConfig struct {
	CalendarHref string `json:"calendar_href"`
	CalendarName string `json:"calendar_name"`
}

// caldavClient builds a CalDAV client from the stored Fastmail credentials.
func (h *integrationHandler) caldavClient(ctx context.Context) (*caldav.Client, error) {
	cfg, err := h.configs.Get(ctx, "fastmail")
	if errors.Is(err, db.ErrNotFound) {
		return nil, errors.New("fastmail not connected — connect Fastmail first")
	}
	if err != nil {
		return nil, err
	}
	var fm struct {
		Email       string `json:"email"`
		AppPassword string `json:"app_password"`
	}
	if err := json.Unmarshal([]byte(cfg.Config), &fm); err != nil {
		return nil, errors.New("malformed fastmail config")
	}
	return caldav.NewClient(caldav.Config{
		BaseURL:  caldav.FastmailBaseURL,
		Username: fm.Email,
		Password: fm.AppPassword,
	})
}

// getCaldavConfig returns the stored CalDAV selection, if any.
func (h *integrationHandler) getCaldavConfig(ctx context.Context) (caldavConfig, db.IntegrationConfig, bool, error) {
	row, err := h.configs.Get(ctx, "caldav")
	if errors.Is(err, db.ErrNotFound) {
		return caldavConfig{}, db.IntegrationConfig{}, false, nil
	}
	if err != nil {
		return caldavConfig{}, db.IntegrationConfig{}, false, err
	}
	var cc caldavConfig
	_ = json.Unmarshal([]byte(row.Config), &cc)
	return cc, row, true, nil
}

func (h *integrationHandler) caldavGet(w http.ResponseWriter, r *http.Request) {
	// Requires Fastmail to be connected (credentials are shared).
	if _, err := h.configs.Get(r.Context(), "fastmail"); errors.Is(err, db.ErrNotFound) {
		respond(w, http.StatusOK, map[string]any{"connected": false})
		return
	}
	cc, row, ok, err := h.getCaldavConfig(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		respond(w, http.StatusOK, map[string]any{"connected": true, "enabled": false})
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"connected":      true,
		"enabled":        row.Enabled,
		"calendar_href":  cc.CalendarHref,
		"calendar_name":  cc.CalendarName,
		"last_synced_at": row.LastSyncedAt,
	})
}

// caldavListCalendars discovers the account's writable calendars for the picker.
func (h *integrationHandler) caldavListCalendars(w http.ResponseWriter, r *http.Request) {
	client, err := h.caldavClient(r.Context())
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	cals, err := client.ListCalendars(r.Context())
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	if cals == nil {
		cals = []caldav.Calendar{}
	}
	respond(w, http.StatusOK, cals)
}

// caldavSelect stores the chosen calendar and enables sync.
func (h *integrationHandler) caldavSelect(w http.ResponseWriter, r *http.Request) {
	var body caldavConfig
	if err := decode(r, &body); err != nil || body.CalendarHref == "" {
		respondError(w, http.StatusBadRequest, "calendar_href is required")
		return
	}
	configJSON, _ := json.Marshal(body)
	if _, err := h.configs.Upsert(r.Context(), uuid.New().String(), "caldav", string(configJSON)); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.configs.SetEnabled(r.Context(), "caldav", true); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusOK, map[string]any{"enabled": true, "calendar_href": body.CalendarHref, "calendar_name": body.CalendarName})
}

func (h *integrationHandler) caldavToggle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := decode(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, _, ok, _ := h.getCaldavConfig(r.Context()); !ok {
		respondError(w, http.StatusBadRequest, "select a calendar first")
		return
	}
	if err := h.configs.SetEnabled(r.Context(), "caldav", body.Enabled); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusOK, map[string]any{"enabled": body.Enabled})
}

// caldavSync pushes all currently-scheduled tasks to the chosen calendar.
func (h *integrationHandler) caldavSync(w http.ResponseWriter, r *http.Request) {
	cc, _, ok, err := h.getCaldavConfig(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok || cc.CalendarHref == "" {
		respondError(w, http.StatusBadRequest, "no calendar selected")
		return
	}
	client, err := h.caldavClient(r.Context())
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	count, err := caldav.SyncAll(r.Context(), client, cc.CalendarHref, h.tasks, h.cfg.AppURL)
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	_ = h.configs.TouchSyncTime(r.Context(), "caldav")
	respond(w, http.StatusOK, map[string]any{"synced": count})
}
