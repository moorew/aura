package api

import (
	"net/http"
	"time"

	"github.com/clevercode/sempa/internal/db"
	"github.com/clevercode/sempa/internal/integrations/unfurl"
)

type unfurlHandler struct {
	store *db.UnfurlStore
}

// How long a cached unfurl (success or failure) is considered fresh.
const unfurlTTL = 14 * 24 * time.Hour

// get returns link-preview metadata for ?url=…, fetching and caching it on a
// miss. Always responds 200 with a LinkUnfurl; ok=false means there was no
// usable metadata (the client falls back to a plain link chip).
func (h *unfurlHandler) get(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("url")
	if raw == "" {
		respondError(w, http.StatusBadRequest, "missing url parameter")
		return
	}

	// Serve from cache while fresh.
	if cached, err := h.store.Get(raw); err == nil {
		if t, perr := time.Parse(time.RFC3339, cached.FetchedAt); perr == nil && time.Since(t) < unfurlTTL {
			respond(w, http.StatusOK, cached)
			return
		}
	}

	meta, status, err := unfurl.Fetch(r.Context(), raw)
	row := &db.LinkUnfurl{
		URL:       raw,
		Status:    status,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err == nil && meta != nil {
		row.Title = meta.Title
		row.Description = meta.Description
		row.ImageURL = meta.ImageURL
		row.SiteName = meta.SiteName
		row.FaviconURL = meta.FaviconURL
		row.OK = meta.Title != "" || meta.ImageURL != ""
	}
	// Cache successes and failures alike (negative cache) so we don't re-hit a
	// dead or slow URL on every render.
	_ = h.store.Upsert(row)
	respond(w, http.StatusOK, row)
}
