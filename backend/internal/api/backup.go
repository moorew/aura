package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/clevercode/sempa/internal/backup"
	"github.com/clevercode/sempa/internal/config"
	"github.com/clevercode/sempa/internal/db"
)

type backupHandler struct {
	svc     *backup.Service
	store   *db.BackupStore
	configs *db.IntegrationConfigStore
	hub     *EventHub
	cfg     config.Config
}

// ── Settings ─────────────────────────────────────────────────────────────────

func (h *backupHandler) getSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.Get(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load backup settings")
		return
	}
	// Redact destination secrets and the passphrase before sending to the client.
	if redacted, err := backup.RedactDestinations(settings.Destinations); err == nil {
		settings.Destinations = redacted
	}
	settings.Passphrase = ""

	runs, _ := h.store.ListRuns(r.Context(), 10)
	driveConnected := false
	if _, err := h.configs.Get(r.Context(), backupDriveType); err == nil {
		driveConnected = true
	}
	respond(w, http.StatusOK, map[string]any{
		"settings":        settings,
		"runs":            runs,
		"drive_connected": driveConnected,
		"google_oauth":    h.cfg.GmailClientID != "",
	})
}

type updateBackupRequest struct {
	Enabled      bool    `json:"enabled"`
	ScheduleHour int     `json:"schedule_hour"`
	Retention    int     `json:"retention"`
	SecurityMode string  `json:"security_mode"`
	Destinations any     `json:"destinations"` // JSON array, stored as-is
	Passphrase   *string `json:"passphrase"`   // omit to keep existing; "" to clear
}

func (h *backupHandler) updateSettings(w http.ResponseWriter, r *http.Request) {
	var req updateBackupRequest
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	switch req.SecurityMode {
	case backup.ModeNone, backup.ModeEncrypt, backup.ModeExcludeSecrets:
	default:
		respondError(w, http.StatusUnprocessableEntity, "invalid security_mode")
		return
	}
	if req.ScheduleHour < 0 || req.ScheduleHour > 23 {
		respondError(w, http.StatusUnprocessableEntity, "schedule_hour must be 0-23")
		return
	}
	if req.Retention < 1 {
		req.Retention = 1
	}

	// Resolve destinations, merging redacted secrets back from the stored copy.
	destJSON, err := mergeDestinations(r.Context(), h.store, req.Destinations)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// Require a passphrase before enabling encryption.
	if req.SecurityMode == backup.ModeEncrypt {
		existing, _ := h.store.Get(r.Context())
		hasNew := req.Passphrase != nil && *req.Passphrase != ""
		if !hasNew && !existing.HasPassphrase {
			respondError(w, http.StatusUnprocessableEntity, "a passphrase is required for encrypted backups")
			return
		}
	}

	if err := h.store.UpdateSettings(r.Context(), req.Enabled, req.ScheduleHour, req.Retention,
		req.SecurityMode, destJSON, req.Passphrase); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save backup settings")
		return
	}
	h.getSettings(w, r)
}

func (h *backupHandler) listRuns(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	runs, err := h.store.ListRuns(r.Context(), limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load run history")
		return
	}
	respond(w, http.StatusOK, runs)
}

// ── Manual download ──────────────────────────────────────────────────────────

// download builds a bundle on the fly using the configured security mode and
// streams it to the client.
func (h *backupHandler) download(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.Get(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load backup settings")
		return
	}
	mode := settings.SecurityMode
	if mode == backup.ModeEncrypt && settings.Passphrase == "" {
		respondError(w, http.StatusUnprocessableEntity, "encryption is on but no passphrase is set")
		return
	}

	result, err := h.svc.Build(r.Context(), mode, settings.Passphrase)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to build backup: "+err.Error())
		return
	}
	defer result.Cleanup()

	f, err := os.Open(result.Path)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to open backup")
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+result.Filename+`"`)
	w.Header().Set("Content-Length", strconv.FormatInt(result.Size, 10))
	http.ServeContent(w, r, result.Filename, modTime(result.Path), f)
}

// ── Restore ──────────────────────────────────────────────────────────────────

// restore wipes all current data and replaces it with the uploaded bundle.
func (h *backupHandler) restore(w http.ResponseWriter, r *http.Request) {
	// Allow large uploads (bundles can be hundreds of MB).
	r.Body = http.MaxBytesReader(w, r.Body, (500<<20)+(64<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "invalid upload")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	passphrase := r.FormValue("passphrase")

	if err := h.svc.Restore(r.Context(), file, passphrase); err != nil {
		respondError(w, http.StatusBadRequest, "restore failed: "+err.Error())
		return
	}

	// Tell every connected client to reload — their world just changed.
	h.hub.Broadcast("task:change", map[string]string{"entity": "restore"})
	h.hub.Broadcast("objective:change", map[string]string{})
	respond(w, http.StatusOK, map[string]string{"status": "restored"})
}

// runNow triggers an immediate backup to all configured destinations.
func (h *backupHandler) runNow(w http.ResponseWriter, r *http.Request) {
	run, err := h.svc.Run(r.Context(), "manual", driveTokenFunc(h.configs, h.cfg))
	if err != nil {
		// The run row is still recorded; surface the error but include the run.
		respond(w, http.StatusOK, map[string]any{"run": run, "error": err.Error()})
		return
	}
	respond(w, http.StatusOK, map[string]any{"run": run})
}

type testDestinationRequest struct {
	ID string `json:"id"`
}

// testDestination verifies connectivity to a configured destination by listing
// its existing backups.
func (h *backupHandler) testDestination(w http.ResponseWriter, r *http.Request) {
	var req testDestinationRequest
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	settings, err := h.store.Get(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load settings")
		return
	}
	dests, err := backup.ParseDestinations(settings.Destinations)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "bad destination config")
		return
	}
	var target *backup.DestConfig
	for i := range dests {
		if dests[i].ID == req.ID {
			target = &dests[i]
			break
		}
	}
	if target == nil {
		respondError(w, http.StatusNotFound, "destination not found")
		return
	}
	dest, err := backup.NewDestination(*target, driveTokenFunc(h.configs, h.cfg))
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	files, err := dest.List(r.Context())
	if err != nil {
		respond(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	respond(w, http.StatusOK, map[string]any{"ok": true, "existing_backups": len(files)})
}

func modTime(path string) time.Time {
	if fi, err := os.Stat(path); err == nil {
		return fi.ModTime()
	}
	return time.Now()
}

// mergeDestinations merges the client-supplied destinations with the stored copy
// so redacted secrets ("" placeholders) are preserved. Full implementation lives
// in destinations.go.
func mergeDestinations(ctx context.Context, store *db.BackupStore, incoming any) (string, error) {
	if incoming == nil {
		existing, err := store.Get(ctx)
		if err != nil {
			return "[]", nil
		}
		return existing.Destinations, nil
	}
	existing, _ := store.Get(ctx)
	return mergeDestinationSecrets(existing.Destinations, incoming)
}

func mergeDestinationSecrets(existingJSON string, incoming any) (string, error) {
	raw, err := json.Marshal(incoming)
	if err != nil {
		return "", err
	}
	merged, err := backup.MergeDestinations(existingJSON, raw)
	if err != nil {
		return "", err
	}
	return merged, nil
}
