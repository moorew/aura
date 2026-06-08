package db

import (
	"context"
	"database/sql"
)

// Tombstone records a deleted entity so offline clients learn to drop their
// local copy when they next pull. See migration 015_sync_tombstones.sql.
type Tombstone struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	DeletedAt  string `json:"deleted_at"`
}

// SyncChanges is the payload returned by GET /api/v1/sync/changes?since=<cursor>.
// It carries every entity created/updated since the cursor, plus tombstones for
// deletions, and a new cursor the client persists for its next pull.
type SyncChanges struct {
	Tasks       []Task          `json:"tasks"`
	Objectives  []Objective     `json:"objectives"`
	Plans       []DailyPlan     `json:"plans"`
	Tags        []TagDefinition `json:"tags"`
	WeekReviews []WeekReview    `json:"week_reviews"`
	Deletions   []Tombstone     `json:"deletions"`
	Cursor      string          `json:"cursor"`
}

// SyncStore powers the pull side of offline sync and records deletions.
type SyncStore struct{ db *sql.DB }

func NewSyncStore(db *sql.DB) *SyncStore { return &SyncStore{db: db} }

// RecordTombstone notes that an entity was deleted so the deletion propagates to
// offline clients on their next pull. Called from delete handlers. A re-created
// entity (same id) overwrites its tombstone via the primary-key upsert.
func (s *SyncStore) RecordTombstone(ctx context.Context, entityType, entityID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sync_tombstones (entity_type, entity_id, deleted_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(entity_type, entity_id) DO UPDATE SET deleted_at = datetime('now')`,
		entityType, entityID)
	return err
}

// ClearTombstone removes a tombstone — call when an entity with the same id is
// re-created, so a stale deletion doesn't later wipe the new row on a client.
func (s *SyncStore) ClearTombstone(ctx context.Context, entityType, entityID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM sync_tombstones WHERE entity_type = ? AND entity_id = ?`,
		entityType, entityID)
	return err
}

// Changes returns everything modified since the given cursor. An empty cursor
// means "everything" (initial full sync). The returned Cursor is the server's
// current time; the client passes it back on the next pull.
//
// Comparisons are lexicographic on the `datetime('now')` text format
// (YYYY-MM-DD HH:MM:SS, UTC, fixed width) which all updated_at columns use, so
// string ordering equals chronological ordering. Resolution is one second;
// same-second concurrent writes are reconciled by the client's idempotent,
// id-keyed last-write-wins upsert and by the live SSE channel.
func (s *SyncStore) Changes(ctx context.Context, since string) (SyncChanges, error) {
	out := SyncChanges{
		Tasks:       []Task{},
		Objectives:  []Objective{},
		Plans:       []DailyPlan{},
		Tags:        []TagDefinition{},
		WeekReviews: []WeekReview{},
		Deletions:   []Tombstone{},
	}

	// Capture the server clock up front; it becomes the next cursor.
	if err := s.db.QueryRowContext(ctx, `SELECT datetime('now')`).Scan(&out.Cursor); err != nil {
		return out, err
	}

	// where builds "WHERE updated_at > ?" with the cursor arg, or no filter on
	// an empty cursor (full sync).
	filter := ""
	args := []any{}
	if since != "" {
		filter = " WHERE updated_at > ?"
		args = append(args, since)
	}

	// Tasks
	if rows, err := s.db.QueryContext(ctx,
		`SELECT `+taskCols+` FROM tasks`+filter+` ORDER BY updated_at`, args...); err != nil {
		return out, err
	} else {
		out.Tasks, err = collectTasks(rows)
		rows.Close()
		if err != nil {
			return out, err
		}
	}

	// Objectives
	if rows, err := s.db.QueryContext(ctx,
		`SELECT `+objCols+` FROM weekly_objectives`+filter+` ORDER BY updated_at`, args...); err != nil {
		return out, err
	} else {
		for rows.Next() {
			o, err := scanObjective(rows)
			if err != nil {
				rows.Close()
				return out, err
			}
			out.Objectives = append(out.Objectives, o)
		}
		rows.Close()
	}

	// Daily plans
	if rows, err := s.db.QueryContext(ctx,
		`SELECT `+planCols+` FROM daily_plans`+filter+` ORDER BY updated_at`, args...); err != nil {
		return out, err
	} else {
		for rows.Next() {
			p, err := scanPlan(rows)
			if err != nil {
				rows.Close()
				return out, err
			}
			out.Plans = append(out.Plans, p)
		}
		rows.Close()
	}

	// Tags
	if rows, err := s.db.QueryContext(ctx,
		`SELECT `+tagCols+` FROM tag_definitions`+filter+` ORDER BY updated_at`, args...); err != nil {
		return out, err
	} else {
		for rows.Next() {
			var t TagDefinition
			if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.CreatedAt, &t.UpdatedAt); err != nil {
				rows.Close()
				return out, err
			}
			out.Tags = append(out.Tags, t)
		}
		rows.Close()
	}

	// Week reviews
	if rows, err := s.db.QueryContext(ctx,
		`SELECT id, week_start, wins, challenges, next_focus, created_at, updated_at
		 FROM week_reviews`+filter+` ORDER BY updated_at`, args...); err != nil {
		return out, err
	} else {
		for rows.Next() {
			r, err := scanWeekReview(rows)
			if err != nil {
				rows.Close()
				return out, err
			}
			out.WeekReviews = append(out.WeekReviews, r)
		}
		rows.Close()
	}

	// Deletions
	delFilter := ""
	delArgs := []any{}
	if since != "" {
		delFilter = " WHERE deleted_at > ?"
		delArgs = append(delArgs, since)
	}
	if rows, err := s.db.QueryContext(ctx,
		`SELECT entity_type, entity_id, deleted_at FROM sync_tombstones`+delFilter+` ORDER BY deleted_at`, delArgs...); err != nil {
		return out, err
	} else {
		for rows.Next() {
			var t Tombstone
			if err := rows.Scan(&t.EntityType, &t.EntityID, &t.DeletedAt); err != nil {
				rows.Close()
				return out, err
			}
			out.Deletions = append(out.Deletions, t)
		}
		rows.Close()
	}

	return out, nil
}
