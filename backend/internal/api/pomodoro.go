package api

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/db"
)

type sessionHandler struct {
	store *db.SessionStore
}

type createSessionRequest struct {
	TaskID          string  `json:"task_id"`
	DurationMinutes int     `json:"duration_minutes"`
	StartedAt       string  `json:"started_at"`
	CompletedAt     *string `json:"completed_at"`
	WasCompleted    bool    `json:"was_completed"`
}

func (h *sessionHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TaskID == "" || req.StartedAt == "" {
		respondError(w, http.StatusUnprocessableEntity, "task_id and started_at are required")
		return
	}
	duration := req.DurationMinutes
	if duration <= 0 {
		duration = 25
	}

	session, err := h.store.Create(r.Context(), db.CreateSessionParams{
		ID:              uuid.New().String(),
		TaskID:          req.TaskID,
		DurationMinutes: duration,
		StartedAt:       req.StartedAt,
		CompletedAt:     req.CompletedAt,
		WasCompleted:    req.WasCompleted,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to record session")
		return
	}
	respond(w, http.StatusCreated, session)
}
