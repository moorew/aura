package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/db"
)

type planHandler struct {
	store *db.DailyPlanStore
}

type upsertPlanRequest struct {
	Status     string  `json:"status"`
	Intention  *string `json:"intention"`
	Reflection *string `json:"reflection"`
	Wins       *string `json:"wins"`
	ShutdownAt *string `json:"shutdown_at"`
}

func (h *planHandler) get(w http.ResponseWriter, r *http.Request) {
	plan, err := h.store.Get(r.Context(), chi.URLParam(r, "date"))
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusNotFound, "plan not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get plan")
		return
	}
	respond(w, http.StatusOK, plan)
}

func (h *planHandler) upsert(w http.ResponseWriter, r *http.Request) {
	date := chi.URLParam(r, "date")

	var req upsertPlanRequest
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	status := req.Status
	if status == "" {
		status = "pending"
	}

	plan, err := h.store.Upsert(r.Context(), db.UpsertPlanParams{
		ID:         uuid.New().String(),
		PlanDate:   date,
		Status:     status,
		Intention:  req.Intention,
		Reflection: req.Reflection,
		Wins:       req.Wins,
		ShutdownAt: req.ShutdownAt,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to upsert plan")
		return
	}
	respond(w, http.StatusOK, plan)
}
