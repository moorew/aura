package api

import (
	"net/http"

	"github.com/clevercode/sempa/internal/db"
)

type syncHandler struct {
	store *db.SyncStore
}

// changes serves GET /api/v1/sync/changes?since=<cursor>. Offline clients call
// this on reconnect to pull everything created/updated/deleted since their last
// successful sync. An empty `since` performs a full initial sync.
func (h *syncHandler) changes(w http.ResponseWriter, r *http.Request) {
	since := r.URL.Query().Get("since")
	changes, err := h.store.Changes(r.Context(), since)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load changes")
		return
	}
	respond(w, http.StatusOK, changes)
}
