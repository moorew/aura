-- Persist login sessions so they survive backend restarts/redeploys.
-- Previously sessions lived only in memory, so every container restart
-- logged every user out (their stored token became unknown to the server).
CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT PRIMARY KEY,
    email      TEXT NOT NULL DEFAULT '',
    expires_at TEXT NOT NULL          -- RFC3339 UTC; lexicographically sortable
);

CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
