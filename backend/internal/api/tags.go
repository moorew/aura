package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/db"
)

var defaultPalette = []string{
	"#3b82f6", // blue
	"#10b981", // emerald
	"#f59e0b", // amber
	"#ef4444", // red
	"#8b5cf6", // violet
	"#ec4899", // pink
	"#06b6d4", // cyan
	"#84cc16", // lime
}

type tagHandler struct{ store *db.TagStore }

func (h *tagHandler) list(w http.ResponseWriter, r *http.Request) {
	tags, err := h.store.List(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusOK, tags)
}

func (h *tagHandler) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := decode(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if body.Color == "" {
		// Auto-assign next palette color
		existing, _ := h.store.List(r.Context())
		body.Color = defaultPalette[len(existing)%len(defaultPalette)]
	}

	tag, err := h.store.Upsert(r.Context(), uuid.New().String(), body.Name, body.Color)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusCreated, tag)
}

func (h *tagHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Color string `json:"color"`
	}
	if err := decode(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	tag, err := h.store.Update(r.Context(), id, body.Color)
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusNotFound, "tag not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusOK, tag)
}

func (h *tagHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.Delete(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			respondError(w, http.StatusNotFound, "tag not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
