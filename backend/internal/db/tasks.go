package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

type Task struct {
	ID                  string   `json:"id"`
	Title               string   `json:"title"`
	Description         *string  `json:"description"`
	PlannedDate         *string  `json:"planned_date"`
	WeekStart           *string  `json:"week_start"`
	Status              string   `json:"status"`
	Position            float64  `json:"position"`
	TimeEstimateMinutes *int64   `json:"time_estimate_minutes"`
	TimeActualMinutes   *int64   `json:"time_actual_minutes"`
	ParentTaskID        *string  `json:"parent_task_id"`
	WeeklyObjectiveID   *string  `json:"weekly_objective_id"`
	Source              *string  `json:"source"`
	SourceID            *string  `json:"source_id"`
	SourceURL           *string  `json:"source_url"`
	SourceMetadata      *string  `json:"source_metadata"`
	CompletedAt         *string  `json:"completed_at"`
	ArchivedAt          *string  `json:"archived_at"`
	CreatedAt           string   `json:"created_at"`
	UpdatedAt           string   `json:"updated_at"`
	// Tags & recurrence (added in migration 002)
	Tags               []string `json:"tags"`
	RecurrenceRule     *string  `json:"recurrence_rule"`
	RecurrenceOriginID *string  `json:"recurrence_origin_id"`
	IsCustomized       bool     `json:"is_customized"`
	// Timeboxing (added in migration 006)
	ScheduledStart *string `json:"scheduled_start"`
	ScheduledEnd   *string `json:"scheduled_end"`
}

const taskCols = `id, title, description, planned_date, week_start, status, position,
       time_estimate_minutes, time_actual_minutes, parent_task_id, weekly_objective_id,
       source, source_id, source_url, source_metadata,
       completed_at, archived_at, created_at, updated_at,
       tags, recurrence_rule, recurrence_origin_id, is_customized,
       scheduled_start, scheduled_end`

func scanTask(s scanner) (Task, error) {
	var t Task
	var description, plannedDate, weekStart sql.NullString
	var timeEst, timeAct sql.NullInt64
	var parentID, objID sql.NullString
	var source, sourceID, sourceURL, sourceMeta sql.NullString
	var completedAt, archivedAt sql.NullString
	var tagsJSON string
	var recurrenceRule, recurrenceOriginID sql.NullString
	var isCustomized int64
	var scheduledStart, scheduledEnd sql.NullString

	err := s.Scan(
		&t.ID, &t.Title, &description, &plannedDate, &weekStart,
		&t.Status, &t.Position,
		&timeEst, &timeAct,
		&parentID, &objID,
		&source, &sourceID, &sourceURL, &sourceMeta,
		&completedAt, &archivedAt,
		&t.CreatedAt, &t.UpdatedAt,
		&tagsJSON, &recurrenceRule, &recurrenceOriginID, &isCustomized,
		&scheduledStart, &scheduledEnd,
	)
	if err != nil {
		return Task{}, err
	}

	t.Description = nullStr(description)
	t.PlannedDate = nullStr(plannedDate)
	t.WeekStart = nullStr(weekStart)
	t.TimeEstimateMinutes = nullInt(timeEst)
	t.TimeActualMinutes = nullInt(timeAct)
	t.ParentTaskID = nullStr(parentID)
	t.WeeklyObjectiveID = nullStr(objID)
	t.Source = nullStr(source)
	t.SourceID = nullStr(sourceID)
	t.SourceURL = nullStr(sourceURL)
	t.SourceMetadata = nullStr(sourceMeta)
	t.CompletedAt = nullStr(completedAt)
	t.ArchivedAt = nullStr(archivedAt)
	t.RecurrenceRule = nullStr(recurrenceRule)
	t.RecurrenceOriginID = nullStr(recurrenceOriginID)
	t.IsCustomized = isCustomized == 1
	t.ScheduledStart = nullStr(scheduledStart)
	t.ScheduledEnd = nullStr(scheduledEnd)

	if tagsJSON != "" && tagsJSON != "[]" {
		_ = json.Unmarshal([]byte(tagsJSON), &t.Tags)
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}

	return t, nil
}

type TaskStore struct{ db *sql.DB }

func NewTaskStore(db *sql.DB) *TaskStore { return &TaskStore{db: db} }

func (s *TaskStore) Get(ctx context.Context, id string) (Task, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+taskCols+` FROM tasks WHERE id = ?`, id)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return t, err
}

func (s *TaskStore) ListByDate(ctx context.Context, date string) ([]Task, error) {
	// Exclude recurring templates (they have no planned_date by design)
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+taskCols+` FROM tasks WHERE planned_date = ? ORDER BY status, position`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTasks(rows)
}

func (s *TaskStore) ListByWeek(ctx context.Context, weekStart string) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+taskCols+` FROM tasks WHERE week_start = ? ORDER BY planned_date, status, position`, weekStart)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTasks(rows)
}

func (s *TaskStore) ListBacklog(ctx context.Context) ([]Task, error) {
	// Exclude recurring templates from backlog
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+taskCols+` FROM tasks
		 WHERE planned_date IS NULL AND recurrence_rule IS NULL
		   AND status NOT IN ('done','cancelled')
		 ORDER BY position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTasks(rows)
}

// ListRecurringTemplates returns all tasks that are recurring templates.
func (s *TaskStore) ListRecurringTemplates(ctx context.Context) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+taskCols+` FROM tasks
		 WHERE recurrence_rule IS NOT NULL AND recurrence_origin_id IS NULL
		 ORDER BY title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTasks(rows)
}

// FindPendingRecurringInstance finds the most recent non-done, non-cancelled, non-in_progress
// instance of a recurring template that hasn't been customised (safe to carry forward).
func (s *TaskStore) FindPendingRecurringInstance(ctx context.Context, originID string) (*Task, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+taskCols+` FROM tasks
		 WHERE recurrence_origin_id = ?
		   AND status IN ('backlog','planned')
		   AND is_customized = 0
		 ORDER BY planned_date DESC
		 LIMIT 1`, originID)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

type CreateTaskParams struct {
	ID                  string
	Title               string
	Description         *string
	PlannedDate         *string
	WeekStart           *string
	Status              string
	Position            float64
	TimeEstimateMinutes *int64
	ParentTaskID        *string
	WeeklyObjectiveID   *string
	Source              *string
	SourceID            *string
	SourceURL           *string
	SourceMetadata      *string
	Tags               []string
	RecurrenceRule     *string
	RecurrenceOriginID *string
	ScheduledStart     *string
	ScheduledEnd       *string
}

func (s *TaskStore) Create(ctx context.Context, p CreateTaskParams) (Task, error) {
	tagsJSON, _ := json.Marshal(p.Tags)
	if tagsJSON == nil {
		tagsJSON = []byte("[]")
	}
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO tasks (
			id, title, description, planned_date, week_start, status, position,
			time_estimate_minutes, parent_task_id, weekly_objective_id,
			source, source_id, source_url, source_metadata,
			tags, recurrence_rule, recurrence_origin_id,
			scheduled_start, scheduled_end
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		RETURNING `+taskCols,
		p.ID, p.Title, p.Description, p.PlannedDate, p.WeekStart, p.Status, p.Position,
		p.TimeEstimateMinutes, p.ParentTaskID, p.WeeklyObjectiveID,
		p.Source, p.SourceID, p.SourceURL, p.SourceMetadata,
		string(tagsJSON), p.RecurrenceRule, p.RecurrenceOriginID,
		p.ScheduledStart, p.ScheduledEnd,
	)
	return scanTask(row)
}

func (s *TaskStore) Update(ctx context.Context, t Task) (Task, error) {
	tagsJSON, _ := json.Marshal(t.Tags)
	if tagsJSON == nil {
		tagsJSON = []byte("[]")
	}
	isCustomized := 0
	if t.IsCustomized {
		isCustomized = 1
	}
	row := s.db.QueryRowContext(ctx, `
		UPDATE tasks SET
			title                 = ?,
			description           = ?,
			status                = ?,
			position              = ?,
			planned_date          = ?,
			week_start            = ?,
			time_estimate_minutes = ?,
			time_actual_minutes   = ?,
			weekly_objective_id   = ?,
			completed_at          = ?,
			tags                  = ?,
			is_customized         = ?,
			scheduled_start       = ?,
			scheduled_end         = ?,
			updated_at            = datetime('now')
		WHERE id = ?
		RETURNING `+taskCols,
		t.Title, t.Description, t.Status, t.Position,
		t.PlannedDate, t.WeekStart,
		t.TimeEstimateMinutes, t.TimeActualMinutes,
		t.WeeklyObjectiveID, t.CompletedAt,
		string(tagsJSON), isCustomized,
		t.ScheduledStart, t.ScheduledEnd,
		t.ID,
	)
	updated, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return updated, err
}

func (s *TaskStore) ListByParent(ctx context.Context, parentID string) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+taskCols+` FROM tasks WHERE parent_task_id = ? ORDER BY position`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (s *TaskStore) ListBySource(ctx context.Context, source string) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+taskCols+` FROM tasks WHERE source = ? AND status != 'cancelled' ORDER BY created_at DESC`,
		source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTasks(rows)
}

func (s *TaskStore) FindBySource(ctx context.Context, source, sourceID string) (Task, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+taskCols+` FROM tasks WHERE source = ? AND source_id = ?`, source, sourceID)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return t, err
}

func (s *TaskStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func collectTasks(rows *sql.Rows) ([]Task, error) {
	var tasks []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	if tasks == nil {
		tasks = []Task{}
	}
	return tasks, rows.Err()
}
