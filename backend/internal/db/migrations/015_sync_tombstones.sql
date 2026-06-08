-- Offline sync support: tombstones so deletions propagate to offline clients.
--
-- When a client pulls changes since a cursor it needs to learn not only what
-- was created/updated (those rows carry updated_at) but also what was DELETED
-- while it was offline. The rows themselves are gone, so we record a tombstone
-- on every delete. Tombstones are tiny (type + id + timestamp) and can be
-- pruned later once all known devices have synced past them.
CREATE TABLE IF NOT EXISTS sync_tombstones (
    entity_type TEXT NOT NULL,        -- 'task' | 'objective' | 'plan' | 'tag' | 'week_review'
    entity_id   TEXT NOT NULL,
    deleted_at  TEXT NOT NULL DEFAULT (datetime('now')),  -- RFC-ish, lexicographically sortable
    PRIMARY KEY (entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_tombstones_deleted_at ON sync_tombstones(deleted_at);
