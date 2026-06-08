package db

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

// newSyncTestDB returns a fresh migrated DB plus the stores the sync tests need.
func newSyncTestDB(t *testing.T) (*SyncStore, *TaskStore) {
	t.Helper()
	dbConn, err := Open(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := Migrate(dbConn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { dbConn.Close() })
	return NewSyncStore(dbConn), NewTaskStore(dbConn)
}

func TestSyncChanges_FullSyncReturnsEverything(t *testing.T) {
	ctx := context.Background()
	sync, tasks := newSyncTestDB(t)

	for i := 0; i < 3; i++ {
		if _, err := tasks.Create(ctx, CreateTaskParams{
			ID: uuid.New().String(), Title: "t", Status: "backlog",
		}); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	changes, err := sync.Changes(ctx, "")
	if err != nil {
		t.Fatalf("changes: %v", err)
	}
	if len(changes.Tasks) != 3 {
		t.Fatalf("full sync: want 3 tasks, got %d", len(changes.Tasks))
	}
	if changes.Cursor == "" {
		t.Fatal("expected a non-empty cursor")
	}
}

func TestSyncChanges_IncrementalSinceCursor(t *testing.T) {
	ctx := context.Background()
	sync, tasks := newSyncTestDB(t)

	// First batch, then grab the cursor.
	if _, err := tasks.Create(ctx, CreateTaskParams{ID: uuid.New().String(), Title: "old", Status: "backlog"}); err != nil {
		t.Fatalf("create old: %v", err)
	}
	first, err := sync.Changes(ctx, "")
	if err != nil {
		t.Fatalf("first changes: %v", err)
	}

	// The cursor is one-second resolution; advancing the clock guarantees the
	// new row sorts strictly after the cursor. SQLite has no sleep, so bump the
	// updated_at of the new row to a clearly-later timestamp instead.
	created, err := tasks.Create(ctx, CreateTaskParams{ID: uuid.New().String(), Title: "new", Status: "backlog"})
	if err != nil {
		t.Fatalf("create new: %v", err)
	}
	if _, err := tasks.db.ExecContext(ctx,
		`UPDATE tasks SET updated_at = datetime('now','+1 day') WHERE id = ?`, created.ID); err != nil {
		t.Fatalf("bump updated_at: %v", err)
	}

	delta, err := sync.Changes(ctx, first.Cursor)
	if err != nil {
		t.Fatalf("delta changes: %v", err)
	}
	if len(delta.Tasks) != 1 {
		t.Fatalf("incremental: want 1 task, got %d", len(delta.Tasks))
	}
	if delta.Tasks[0].Title != "new" {
		t.Fatalf("incremental: want the new task, got %q", delta.Tasks[0].Title)
	}
}

func TestSyncChanges_TombstoneSurfacesDeletion(t *testing.T) {
	ctx := context.Background()
	sync, tasks := newSyncTestDB(t)

	created, err := tasks.Create(ctx, CreateTaskParams{ID: uuid.New().String(), Title: "doomed", Status: "backlog"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := tasks.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := sync.RecordTombstone(ctx, "task", created.ID); err != nil {
		t.Fatalf("tombstone: %v", err)
	}

	changes, err := sync.Changes(ctx, "")
	if err != nil {
		t.Fatalf("changes: %v", err)
	}
	if len(changes.Deletions) != 1 {
		t.Fatalf("want 1 deletion, got %d", len(changes.Deletions))
	}
	if got := changes.Deletions[0]; got.EntityType != "task" || got.EntityID != created.ID {
		t.Fatalf("unexpected tombstone: %+v", got)
	}
}

func TestSyncChanges_ClearTombstoneOnRecreate(t *testing.T) {
	ctx := context.Background()
	sync, _ := newSyncTestDB(t)

	id := uuid.New().String()
	if err := sync.RecordTombstone(ctx, "task", id); err != nil {
		t.Fatalf("tombstone: %v", err)
	}
	if err := sync.ClearTombstone(ctx, "task", id); err != nil {
		t.Fatalf("clear: %v", err)
	}
	changes, err := sync.Changes(ctx, "")
	if err != nil {
		t.Fatalf("changes: %v", err)
	}
	if len(changes.Deletions) != 0 {
		t.Fatalf("want 0 deletions after clear, got %d", len(changes.Deletions))
	}
}
