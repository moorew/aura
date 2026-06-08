package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}

func decode(r *http.Request, v any) error {
	// Limit request bodies to 1 MB to prevent memory exhaustion
	limited := http.MaxBytesReader(nil, r.Body, 1<<20)
	return json.NewDecoder(limited).Decode(v)
}

func mondayOfDate(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7
	}
	return t.AddDate(0, 0, -(wd - 1)).Format("2006-01-02")
}

func newID() string {
	return uuid.New().String()
}

// clientOrNewID returns the client-supplied id when it is a valid UUID, else a
// fresh one. Offline clients generate the id locally so it survives sync (the
// local row and the server row share one id, keeping foreign keys stable).
func clientOrNewID(id string) string {
	if id != "" {
		if _, err := uuid.Parse(id); err == nil {
			return id
		}
	}
	return newID()
}
