CREATE TABLE IF NOT EXISTS device_tokens (
    id         TEXT PRIMARY KEY,
    token      TEXT NOT NULL UNIQUE,
    platform   TEXT NOT NULL DEFAULT 'android',  -- android, web
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS notification_log (
    id         TEXT PRIMARY KEY,
    token_id   TEXT NOT NULL REFERENCES device_tokens(id) ON DELETE CASCADE,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL,
    data       TEXT,  -- JSON payload
    status     TEXT NOT NULL DEFAULT 'sent',  -- sent, failed
    error_msg  TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
