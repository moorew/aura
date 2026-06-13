package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/clevercode/sempa/internal/db"
)

type objectiveHandler struct {
	store  *db.ObjectiveStore
	hub    *EventHub
	attach *attachmentHandler // for cascading attachment cleanup on delete
	sync   *db.SyncStore      // records tombstones so deletes propagate offline
}

type createObjectiveRequest struct {
	ID          string  `json:"id"`
	WeekStart   string  `json:"week_start"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Status      string  `json:"status"`
	Position    float64 `json:"position"`
}

type updateObjectiveRequest struct {
	Title       *string  `json:"title"`
	Description *string  `json:"description"`
	Status      *string  `json:"status"`
	Position    *float64 `json:"position"`
	// WeekStart lets an objective be re-planned into another week (e.g. carrying
	// an unfinished objective forward to next week from the weekly review).
	WeekStart *string `json:"week_start"`
}

func (h *objectiveHandler) list(w http.ResponseWriter, r *http.Request) {
	weekStart := r.URL.Query().Get("week_start")
	if weekStart == "" {
		respondError(w, http.StatusBadRequest, "week_start query param is required")
		return
	}
	objs, err := h.store.ListByWeek(r.Context(), weekStart)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list objectives")
		return
	}
	respond(w, http.StatusOK, objs)
}

func (h *objectiveHandler) get(w http.ResponseWriter, r *http.Request) {
	obj, err := h.store.Get(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusNotFound, "objective not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get objective")
		return
	}
	respond(w, http.StatusOK, obj)
}

func (h *objectiveHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createObjectiveRequest
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" || req.WeekStart == "" {
		respondError(w, http.StatusUnprocessableEntity, "title and week_start are required")
		return
	}
	status := req.Status
	if status == "" {
		status = "active"
	}
	obj, err := h.store.Create(r.Context(), db.CreateObjectiveParams{
		ID:          clientOrNewID(req.ID),
		WeekStart:   req.WeekStart,
		Title:       req.Title,
		Description: req.Description,
		Status:      status,
		Position:    req.Position,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create objective")
		return
	}
	if h.sync != nil {
		_ = h.sync.ClearTombstone(r.Context(), "objective", obj.ID)
	}
	h.hub.Broadcast("objective:change", map[string]string{"week_start": obj.WeekStart})
	respond(w, http.StatusCreated, obj)
}

func (h *objectiveHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	obj, err := h.store.Get(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusNotFound, "objective not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get objective")
		return
	}

	var req updateObjectiveRequest
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Title != nil {
		obj.Title = *req.Title
	}
	if req.Description != nil {
		obj.Description = req.Description
	}
	if req.Status != nil {
		obj.Status = *req.Status
	}
	if req.Position != nil {
		obj.Position = *req.Position
	}
	// Remember the original week so a cross-week move refreshes BOTH weeks.
	prevWeek := obj.WeekStart
	if req.WeekStart != nil && *req.WeekStart != "" {
		obj.WeekStart = *req.WeekStart
	}

	updated, err := h.store.Update(r.Context(), obj)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update objective")
		return
	}
	h.hub.Broadcast("objective:change", map[string]string{"week_start": updated.WeekStart})
	if prevWeek != updated.WeekStart {
		h.hub.Broadcast("objective:change", map[string]string{"week_start": prevWeek})
	}
	respond(w, http.StatusOK, updated)
}

func (h *objectiveHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := h.store.Delete(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusNotFound, "objective not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete objective")
		return
	}
	if h.attach != nil {
		h.attach.removeForOwner(r, "objective", id)
	}
	if h.sync != nil {
		_ = h.sync.RecordTombstone(r.Context(), "objective", id)
	}
	h.hub.Broadcast("objective:change", map[string]string{})
	respond(w, http.StatusNoContent, nil)
}
