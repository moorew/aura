-- Fastmail JMAP calendar events (synced bidirectionally)
CREATE TABLE IF NOT EXISTS fastmail_cal_events (
    id          TEXT PRIMARY KEY,
    uid         TEXT NOT NULL UNIQUE,
    summary     TEXT NOT NULL DEFAULT '',
    description TEXT,
    location    TEXT,
    start_time  TEXT NOT NULL,
    end_time    TEXT NOT NULL,
    all_day     INTEGER NOT NULL DEFAULT 0,
    color       TEXT NOT NULL DEFAULT '#6b7280',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_fmcal_start ON fastmail_cal_events(start_time);
