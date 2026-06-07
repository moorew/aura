-- Backup configuration (single row) and run history.
CREATE TABLE IF NOT EXISTS backup_settings (
    id             INTEGER PRIMARY KEY CHECK (id = 1),
    enabled        INTEGER NOT NULL DEFAULT 0,
    schedule_hour  INTEGER NOT NULL DEFAULT 3,        -- local hour (0-23) for the daily run
    retention      INTEGER NOT NULL DEFAULT 7,        -- keep N most recent backups per destination
    security_mode  TEXT    NOT NULL DEFAULT 'none',   -- 'none' | 'encrypt' | 'exclude_secrets'
    passphrase     TEXT,                              -- used to encrypt automated backups (never included in a backup)
    destinations   TEXT    NOT NULL DEFAULT '[]',     -- JSON array of destination configs
    last_run_at    TEXT,
    last_status    TEXT,                              -- 'success' | 'error'
    last_error     TEXT,
    updated_at     TEXT    NOT NULL DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO backup_settings (id) VALUES (1);

CREATE TABLE IF NOT EXISTS backup_runs (
    id           TEXT PRIMARY KEY,
    started_at   TEXT NOT NULL DEFAULT (datetime('now')),
    finished_at  TEXT,
    trigger      TEXT NOT NULL DEFAULT 'manual',      -- 'manual' | 'scheduled'
    status       TEXT NOT NULL,                       -- 'success' | 'error'
    size_bytes   INTEGER,
    filename     TEXT,
    destinations TEXT,                                -- JSON: per-destination result
    error        TEXT
);

CREATE INDEX IF NOT EXISTS idx_backup_runs_started ON backup_runs(started_at);
