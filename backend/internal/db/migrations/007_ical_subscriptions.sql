-- ICS / webcal calendar subscriptions (read-only external calendars)
CREATE TABLE IF NOT EXISTS ical_subscriptions (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL,
    url            TEXT NOT NULL,
    color          TEXT NOT NULL DEFAULT '#6b7280',
    last_synced_at TEXT,
    error_msg      TEXT,
    created_at     TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at     TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS ical_events (
    id              TEXT PRIMARY KEY,
    subscription_id TEXT NOT NULL REFERENCES ical_subscriptions(id) ON DELETE CASCADE,
    uid             TEXT NOT NULL,
    summary         TEXT NOT NULL,
    description     TEXT,
    location        TEXT,
    start_time      TEXT NOT NULL,
    end_time        TEXT NOT NULL,
    all_day         INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(subscription_id, uid)
);

CREATE INDEX IF NOT EXISTS idx_ical_events_sub  ON ical_events(subscription_id);
CREATE INDEX IF NOT EXISTS idx_ical_events_time ON ical_events(start_time);
