-- File attachments for tasks and weekly objectives.
-- Blobs live on the filesystem (see internal/attach); this table holds metadata only.
-- owner_type is polymorphic ('task' | 'objective') so there is no FK constraint;
-- orphan rows + blobs are cleaned up when the owner is deleted (handled in the API layer).
CREATE TABLE IF NOT EXISTS attachments (
    id          TEXT PRIMARY KEY,
    owner_type  TEXT NOT NULL CHECK(owner_type IN ('task', 'objective')),
    owner_id    TEXT NOT NULL,
    filename    TEXT NOT NULL,
    mime_type   TEXT NOT NULL DEFAULT 'application/octet-stream',
    size_bytes  INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_attachments_owner ON attachments(owner_type, owner_id);
