/**
 * Local SQLite schema for the Capacitor (Android) driver.
 *
 * Tauri desktop runs migrations through tauri-plugin-sql (see src-tauri/src/db.rs);
 * Capacitor has no migration runner, so the Android driver applies this script on
 * every open. It must stay IDEMPOTENT (CREATE TABLE/INDEX IF NOT EXISTS) and must
 * match the Rust migrations' resulting schema column-for-column, since both feed
 * the same shared queries in local-api.ts.
 */

export const LOCAL_SCHEMA_SQL = `
CREATE TABLE IF NOT EXISTS weekly_objectives (
    id TEXT PRIMARY KEY,
    week_start TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'completed', 'cancelled')),
    position REAL NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    planned_date TEXT,
    week_start TEXT,
    status TEXT NOT NULL DEFAULT 'planned'
        CHECK (status IN ('backlog', 'planned', 'in_progress', 'done', 'cancelled')),
    position REAL NOT NULL DEFAULT 0,
    time_estimate_minutes INTEGER,
    time_actual_minutes INTEGER,
    parent_task_id TEXT REFERENCES tasks(id) ON DELETE SET NULL,
    weekly_objective_id TEXT REFERENCES weekly_objectives(id) ON DELETE SET NULL,
    source TEXT DEFAULT 'manual',
    source_id TEXT,
    source_url TEXT,
    source_metadata TEXT,
    completed_at TEXT,
    archived_at TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    tags TEXT DEFAULT '[]',
    recurrence_rule TEXT,
    recurrence_origin_id TEXT REFERENCES tasks(id) ON DELETE SET NULL,
    is_customized INTEGER NOT NULL DEFAULT 0,
    scheduled_start TEXT,
    scheduled_end TEXT,
    roughly_at TEXT,
    remind_at TEXT,
    UNIQUE(source, source_id)
);

CREATE TABLE IF NOT EXISTS daily_plans (
    id TEXT PRIMARY KEY,
    plan_date TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'planning', 'active', 'shutdown_complete')),
    intention TEXT,
    reflection TEXT,
    wins TEXT,
    shutdown_at TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS pomodoro_sessions (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    duration_minutes INTEGER NOT NULL DEFAULT 25,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    was_completed INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS tag_definitions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    color TEXT DEFAULT '#6366f1',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS week_reviews (
    id TEXT PRIMARY KEY,
    week_start TEXT NOT NULL UNIQUE,
    wins TEXT,
    challenges TEXT,
    next_focus TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS sync_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('create', 'update', 'delete')),
    payload TEXT NOT NULL DEFAULT '{}',
    synced INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS sync_state (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_tasks_planned_date ON tasks(planned_date);
CREATE INDEX IF NOT EXISTS idx_tasks_week_start ON tasks(week_start);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_objectives_week ON weekly_objectives(week_start);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tag_name ON tag_definitions(lower(name));
CREATE INDEX IF NOT EXISTS idx_sync_pending ON sync_log(synced) WHERE synced = 0;
`;
