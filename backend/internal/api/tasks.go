package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/db"
)

type taskHandler struct {
	store *db.TaskStore
	tags  *db.TagStore
}

type createTaskRequest struct {
	Title               string   `json:"title"`
	Description         *string  `json:"description"`
	PlannedDate         *string  `json:"planned_date"`
	WeekStart           *string  `json:"week_start"`
	Status              string   `json:"status"`
	Position            float64  `json:"position"`
	TimeEstimateMinutes *int64   `json:"time_estimate_minutes"`
	ParentTaskID        *string  `json:"parent_task_id"`
	WeeklyObjectiveID   *string  `json:"weekly_objective_id"`
	Source              *string  `json:"source"`
	SourceID            *string  `json:"source_id"`
	SourceURL           *string  `json:"source_url"`
	SourceMetadata      *string  `json:"source_metadata"`
	Tags                []string `json:"tags"`
	RecurrenceRule      *string  `json:"recurrence_rule"`
}

type updateTaskRequest struct {
	Title               *string  `json:"title"`
	Description         *string  `json:"description"`
	Status              *string  `json:"status"`
	Position            *float64 `json:"position"`
	PlannedDate         *string  `json:"planned_date"`
	WeekStart           *string  `json:"week_start"`
	TimeEstimateMinutes *int64   `json:"time_estimate_minutes"`
	TimeActualMinutes   *int64   `json:"time_actual_minutes"`
	WeeklyObjectiveID   *string  `json:"weekly_objective_id"`
	CompletedAt         *string  `json:"completed_at"`
	Tags                []string `json:"tags"`
}

func (h *taskHandler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	date := q.Get("date")
	weekStart := q.Get("week_start")

	// Generate recurring instances for the requested date before returning
	if date != "" {
		_ = h.store.GenerateForDate(r.Context(), date)
	}

	var (
		tasks []db.Task
		err   error
	)
	switch {
	case date != "":
		tasks, err = h.store.ListByDate(r.Context(), date)
	case weekStart != "":
		tasks, err = h.store.ListByWeek(r.Context(), weekStart)
	default:
		tasks, err = h.store.ListBacklog(r.Context())
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}
	respond(w, http.StatusOK, tasks)
}

func (h *taskHandler) get(w http.ResponseWriter, r *http.Request) {
	task, err := h.store.Get(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get task")
		return
	}
	respond(w, http.StatusOK, task)
}

func (h *taskHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createTaskRequest
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		respondError(w, http.StatusUnprocessableEntity, "title is required")
		return
	}

	status := req.Status
	if status == "" {
		if req.PlannedDate != nil {
			status = "planned"
		} else {
			status = "backlog"
		}
	}

	if req.Tags == nil {
		req.Tags = []string{}
	}

	// Auto-create tag definitions for any new tags
	if h.tags != nil && len(req.Tags) > 0 {
		_ = h.tags.BulkEnsure(r.Context(), req.Tags, defaultPalette)
	}

	task, err := h.store.Create(r.Context(), db.CreateTaskParams{
		ID:                  uuid.New().String(),
		Title:               req.Title,
		Description:         req.Description,
		PlannedDate:         req.PlannedDate,
		WeekStart:           req.WeekStart,
		Status:              status,
		Position:            req.Position,
		TimeEstimateMinutes: req.TimeEstimateMinutes,
		ParentTaskID:        req.ParentTaskID,
		WeeklyObjectiveID:   req.WeeklyObjectiveID,
		Source:              req.Source,
		SourceID:            req.SourceID,
		SourceURL:           req.SourceURL,
		SourceMetadata:      req.SourceMetadata,
		Tags:                req.Tags,
		RecurrenceRule:      req.RecurrenceRule,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create task")
		return
	}

	// If this is a recurring template, immediately generate today's instance
	if req.RecurrenceRule != nil && *req.RecurrenceRule != "" {
		today := time.Now().Format("2006-01-02")
		_ = h.store.GenerateForDate(r.Context(), today)
	}

	respond(w, http.StatusCreated, task)
}

func (h *taskHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	task, err := h.store.Get(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get task")
		return
	}

	var req updateTaskRequest
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Track whether meaningful content changed on a recurring instance
	contentChanged := false

	if req.Title != nil {
		if task.RecurrenceOriginID != nil && *req.Title != task.Title {
			contentChanged = true
		}
		task.Title = *req.Title
	}
	if req.Description != nil {
		if task.RecurrenceOriginID != nil {
			contentChanged = true
		}
		task.Description = req.Description
	}
	if req.Tags != nil {
		task.Tags = req.Tags
		if h.tags != nil && len(req.Tags) > 0 {
			_ = h.tags.BulkEnsure(r.Context(), req.Tags, defaultPalette)
		}
	}
	if req.Status != nil {
		task.Status = *req.Status
	}
	if req.Position != nil {
		task.Position = *req.Position
	}
	if req.PlannedDate != nil {
		task.PlannedDate = req.PlannedDate
	}
	if req.WeekStart != nil {
		task.WeekStart = req.WeekStart
	}
	if req.TimeEstimateMinutes != nil {
		task.TimeEstimateMinutes = req.TimeEstimateMinutes
	}
	if req.TimeActualMinutes != nil {
		task.TimeActualMinutes = req.TimeActualMinutes
	}
	if req.WeeklyObjectiveID != nil {
		task.WeeklyObjectiveID = req.WeeklyObjectiveID
	}
	if req.CompletedAt != nil {
		task.CompletedAt = req.CompletedAt
	}

	// Auto-stamp completed_at when moving to done for the first time
	if req.Status != nil && *req.Status == "done" && task.CompletedAt == nil {
		now := time.Now().UTC().Format(time.RFC3339)
		task.CompletedAt = &now
	}

	// Mark as customised if content changed on a recurring instance
	if contentChanged {
		task.IsCustomized = true
	}

	updated, err := h.store.Update(r.Context(), task)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update task")
		return
	}
	respond(w, http.StatusOK, updated)
}

func (h *taskHandler) listTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := h.store.ListRecurringTemplates(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusOK, templates)
}

func (h *taskHandler) delete(w http.ResponseWriter, r *http.Request) {
	err := h.store.Delete(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete task")
		return
	}
	respond(w, http.StatusNoContent, nil)
}
