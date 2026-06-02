-- Add task_inbox to integration_configs.type.
PRAGMA foreign_keys = OFF;

CREATE TABLE integration_configs_new (
    id             TEXT PRIMARY KEY,
    type           TEXT NOT NULL UNIQUE
                       CHECK(type IN ('gmail','google_calendar','fastmail','jira','task_inbox')),
    enabled        INTEGER NOT NULL DEFAULT 1,
    config         TEXT NOT NULL DEFAULT '{}',
    last_synced_at TEXT,
    created_at     TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at     TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO integration_configs_new SELECT * FROM integration_configs;
DROP TABLE integration_configs;
ALTER TABLE integration_configs_new RENAME TO integration_configs;

PRAGMA foreign_keys = ON;
