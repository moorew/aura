package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/db"
	"github.com/clevercode/sempa/internal/integrations/gmail"
	"github.com/clevercode/sempa/internal/integrations/jira"
)

type taskHandler struct {
	store   *db.TaskStore
	tags    *db.TagStore
	configs *db.IntegrationConfigStore // for calendar write-back
	appURL  string                     // base URL for task links
	hub     *EventHub
	attach  *attachmentHandler // for cascading attachment cleanup on delete
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
	RoughlyAt           *string  `json:"roughly_at"`
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
	ParentTaskID        *string  `json:"parent_task_id"`
	ScheduledStart      *string  `json:"scheduled_start"`
	ScheduledEnd        *string  `json:"scheduled_end"`
	RoughlyAt           *string  `json:"roughly_at"`
}

func (h *taskHandler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	date := q.Get("date")
	weekStart := q.Get("week_start")

	// Generate recurring instances before returning results. `today` is the
	// client's local date — passing it keeps rollover correct across timezones.
	if weekStart != "" {
		_ = h.store.GenerateForWeek(r.Context(), weekStart, q.Get("today"))
	} else if date != "" {
		_ = h.store.GenerateForDate(r.Context(), date)
	}

	parentID := q.Get("parent_id")
	source := q.Get("source")
	recurrenceOrigin := q.Get("recurrence_origin")

	var (
		tasks []db.Task
		err   error
	)
	switch {
	case parentID != "":
		tasks, err = h.store.ListByParent(r.Context(), parentID)
	case recurrenceOrigin != "":
		tasks, err = h.store.ListByRecurrenceOrigin(r.Context(), recurrenceOrigin)
	case source != "":
		tasks, err = h.store.ListBySource(r.Context(), source)
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
		RoughlyAt:           req.RoughlyAt,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create task")
		return
	}

	// Adding a sub-task to a recurring instance counts as a modification, so the
	// instance is no longer "pristine" and must survive rollover (carry forward).
	if req.ParentTaskID != nil && *req.ParentTaskID != "" {
		h.markRecurringInstanceModified(r.Context(), *req.ParentTaskID)
	}

	// If this is a recurring template, immediately generate today's instance
	if req.RecurrenceRule != nil && *req.RecurrenceRule != "" {
		today := time.Now().Format("2006-01-02")
		_ = h.store.GenerateForDate(r.Context(), today)
	}

	meta := map[string]string{"entity": "task"}
	if task.PlannedDate != nil {
		meta["date"] = *task.PlannedDate
	}
	if task.WeekStart != nil {
		meta["week_start"] = *task.WeekStart
	}
	h.hub.Broadcast("task:change", meta)
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
	if req.ParentTaskID != nil {
		task.ParentTaskID = req.ParentTaskID
	}
	if req.ScheduledStart != nil {
		task.ScheduledStart = req.ScheduledStart
	}
	if req.ScheduledEnd != nil {
		task.ScheduledEnd = req.ScheduledEnd
	}
	if req.RoughlyAt != nil {
		if task.RecurrenceOriginID != nil {
			contentChanged = true
		}
		task.RoughlyAt = req.RoughlyAt
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

	// Checking off / editing a sub-task of a recurring instance modifies that
	// instance, so it should carry forward rather than be replaced on rollover.
	if updated.ParentTaskID != nil && *updated.ParentTaskID != "" {
		h.markRecurringInstanceModified(r.Context(), *updated.ParentTaskID)
	}

	// Write focus block to Google Calendar when a task gets a scheduled time
	if req.ScheduledStart != nil && *req.ScheduledStart != "" &&
		(task.Source == nil || *task.Source != "google_calendar") &&
		h.configs != nil {
		go h.writeFocusBlock(updated)
	}

	// Jira writeback: close the linked ticket when task is marked done
	if req.Status != nil && *req.Status == "done" &&
		updated.Source != nil && *updated.Source == "jira" &&
		updated.SourceID != nil && h.configs != nil {
		go h.writeJiraTransition(updated)
	}

	meta := map[string]string{"entity": "task"}
	if updated.PlannedDate != nil {
		meta["date"] = *updated.PlannedDate
	}
	if updated.WeekStart != nil {
		meta["week_start"] = *updated.WeekStart
	}
	h.hub.Broadcast("task:change", meta)
	respond(w, http.StatusOK, updated)
}

// markRecurringInstanceModified flags a parent task as customised when it is a
// recurring instance, so the smart rollover carries it forward instead of
// deleting it. No-op for normal (non-recurring) parents.
func (h *taskHandler) markRecurringInstanceModified(ctx context.Context, parentID string) {
	parent, err := h.store.Get(ctx, parentID)
	if err != nil || parent.RecurrenceOriginID == nil || parent.IsCustomized {
		return
	}
	parent.IsCustomized = true
	_, _ = h.store.Update(ctx, parent)
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
	id := chi.URLParam(r, "id")
	err := h.store.Delete(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete task")
		return
	}
	if h.attach != nil {
		h.attach.removeForOwner(r, "task", id)
	}
	h.hub.Broadcast("task:change", map[string]string{"entity": "task"})
	respond(w, http.StatusNoContent, nil)
}

// writeJiraTransition closes the linked Jira issue when a task is marked done.
// Runs in a goroutine; errors are silently ignored.
func (h *taskHandler) writeJiraTransition(task db.Task) {
	cfg, err := h.configs.Get(context.Background(), "jira")
	if err != nil {
		return
	}
	var jiraCfg jira.Config
	if err := json.Unmarshal([]byte(cfg.Config), &jiraCfg); err != nil {
		return
	}
	client := jira.NewClient(jiraCfg)
	_ = client.TransitionToDone(context.Background(), *task.SourceID)
}

// writeFocusBlock creates a Google Calendar event for a newly-scheduled task.
// Runs in a goroutine; errors are silently ignored (graceful degradation).
func (h *taskHandler) writeFocusBlock(task db.Task) {
	if task.ScheduledStart == nil || task.ScheduledEnd == nil {
		return
	}
	cfg, err := h.configs.Get(context.Background(), "gmail")
	if err != nil {
		return
	}
	var stored gmail.StoredToken
	if err := json.Unmarshal([]byte(cfg.Config), &stored); err != nil {
		return
	}
	if !stored.CalendarEnabled {
		return
	}
	if err := gmail.RefreshAccessToken(context.Background(),
		"", "", &stored); err != nil {
		return // can't refresh, skip
	}
	calID := "primary"
	taskURL := h.appURL + "/task/" + task.ID
	_, _ = gmail.WriteFocusBlock(context.Background(),
		stored.AccessToken, calID,
		task.Title, *task.ScheduledStart, *task.ScheduledEnd, taskURL)
}
