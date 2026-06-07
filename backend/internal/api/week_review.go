package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/db"
)

type weekReviewHandler struct {
	store *db.WeekReviewStore
}

type upsertWeekReviewRequest struct {
	Wins       *string `json:"wins"`
	Challenges *string `json:"challenges"`
	NextFocus  *string `json:"next_focus"`
}

// list returns all week reviews newest-first, for the Journal timeline.
// Optional ?limit=N.
func (h *weekReviewHandler) list(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	reviews, err := h.store.List(r.Context(), limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list reviews")
		return
	}
	respond(w, http.StatusOK, reviews)
}

func (h *weekReviewHandler) get(w http.ResponseWriter, r *http.Request) {
	weekStart := chi.URLParam(r, "weekStart")
	review, err := h.store.Get(r.Context(), weekStart)
	if errors.Is(err, db.ErrNotFound) {
		// Return empty review stub so frontend doesn't need special-case handling
		respond(w, http.StatusOK, db.WeekReview{WeekStart: weekStart})
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load review")
		return
	}
	respond(w, http.StatusOK, review)
}

func (h *weekReviewHandler) upsert(w http.ResponseWriter, r *http.Request) {
	weekStart := chi.URLParam(r, "weekStart")
	var req upsertWeekReviewRequest
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	review, err := h.store.Upsert(r.Context(), uuid.New().String(), weekStart, req.Wins, req.Challenges, req.NextFocus)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save review")
		return
	}
	respond(w, http.StatusOK, review)
}
