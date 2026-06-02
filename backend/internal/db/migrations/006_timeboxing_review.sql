-- Timeboxing: scheduled time slots on tasks
ALTER TABLE tasks ADD COLUMN scheduled_start TEXT;
ALTER TABLE tasks ADD COLUMN scheduled_end   TEXT;

-- Weekly review ritual
CREATE TABLE IF NOT EXISTS week_reviews (
    id                   TEXT PRIMARY KEY,
    week_start           TEXT NOT NULL UNIQUE,
    wins                 TEXT,   -- JSON array of strings
    challenges           TEXT,   -- JSON array of strings
    next_focus           TEXT,   -- free-text intention for next week
    created_at           TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at           TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_week_reviews_week_start ON week_reviews(week_start);
