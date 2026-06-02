-- Expand tasks.source to include 'email_forward' for inbound SMTP tasks.
PRAGMA foreign_keys = OFF;

CREATE TABLE tasks_new (
    id                   TEXT PRIMARY KEY,
    title                TEXT NOT NULL,
    description          TEXT,
    planned_date         TEXT,
    week_start           TEXT,
    status               TEXT NOT NULL DEFAULT 'backlog'
                             CHECK(status IN ('backlog','planned','in_progress','done','cancelled')),
    position             REAL NOT NULL DEFAULT 0,
    time_estimate_minutes INTEGER,
    time_actual_minutes  INTEGER,
    parent_task_id       TEXT REFERENCES tasks_new(id) ON DELETE SET NULL,
    weekly_objective_id  TEXT REFERENCES weekly_objectives(id) ON DELETE SET NULL,
    source               TEXT CHECK(source IN ('manual','gmail','google_calendar','fastmail','jira','email_forward')),
    source_id            TEXT,
    source_url           TEXT,
    source_metadata      TEXT,
    completed_at         TEXT,
    archived_at          TEXT,
    created_at           TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at           TEXT NOT NULL DEFAULT (datetime('now')),
    tags                 TEXT NOT NULL DEFAULT '[]',
    recurrence_rule      TEXT,
    recurrence_origin_id TEXT REFERENCES tasks_new(id) ON DELETE SET NULL,
    is_customized        INTEGER NOT NULL DEFAULT 0,
    UNIQUE(source, source_id)
);

INSERT INTO tasks_new SELECT * FROM tasks;
DROP TABLE tasks;
ALTER TABLE tasks_new RENAME TO tasks;

CREATE INDEX IF NOT EXISTS idx_tasks_planned_date      ON tasks(planned_date);
CREATE INDEX IF NOT EXISTS idx_tasks_week_start        ON tasks(week_start);
CREATE INDEX IF NOT EXISTS idx_tasks_status            ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_weekly_objective  ON tasks(weekly_objective_id);
CREATE INDEX IF NOT EXISTS idx_tasks_recurrence_origin ON tasks(recurrence_origin_id);

PRAGMA foreign_keys = ON;
