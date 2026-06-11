use tauri::AppHandle;
use tauri_plugin_sql::{Migration, MigrationKind};

/// Returns migrations matching the Go backend's SQLite schema, plus
/// the local sync_log table for offline-first mutation queuing.
pub fn get_migrations() -> Vec<Migration> {
    vec![
        Migration {
            version: 1,
            description: "initial schema",
            sql: r#"
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

                CREATE TABLE IF NOT EXISTS integration_configs (
                    id TEXT PRIMARY KEY,
                    type TEXT NOT NULL UNIQUE,
                    enabled INTEGER NOT NULL DEFAULT 1,
                    config TEXT NOT NULL DEFAULT '{}',
                    last_synced_at TEXT,
                    created_at TEXT NOT NULL DEFAULT (datetime('now')),
                    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
                );

                CREATE INDEX IF NOT EXISTS idx_tasks_planned_date ON tasks(planned_date);
                CREATE INDEX IF NOT EXISTS idx_tasks_week_start ON tasks(week_start);
                CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
                CREATE INDEX IF NOT EXISTS idx_objectives_week ON weekly_objectives(week_start);
            "#,
            kind: MigrationKind::Up,
        },
        Migration {
            version: 2,
            description: "tags and recurrence",
            sql: r#"
                ALTER TABLE tasks ADD COLUMN tags TEXT DEFAULT '[]';
                ALTER TABLE tasks ADD COLUMN recurrence_rule TEXT;
                ALTER TABLE tasks ADD COLUMN recurrence_origin_id TEXT REFERENCES tasks(id) ON DELETE SET NULL;
                ALTER TABLE tasks ADD COLUMN is_customized INTEGER NOT NULL DEFAULT 0;

                CREATE TABLE IF NOT EXISTS tag_definitions (
                    id TEXT PRIMARY KEY,
                    name TEXT NOT NULL,
                    color TEXT DEFAULT '#6366f1',
                    created_at TEXT NOT NULL DEFAULT (datetime('now')),
                    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
                );
                CREATE UNIQUE INDEX IF NOT EXISTS idx_tag_name ON tag_definitions(lower(name));
            "#,
            kind: MigrationKind::Up,
        },
        Migration {
            version: 3,
            description: "timeboxing and week reviews",
            sql: r#"
                ALTER TABLE tasks ADD COLUMN scheduled_start TEXT;
                ALTER TABLE tasks ADD COLUMN scheduled_end TEXT;

                CREATE TABLE IF NOT EXISTS week_reviews (
                    id TEXT PRIMARY KEY,
                    week_start TEXT NOT NULL UNIQUE,
                    wins TEXT,
                    challenges TEXT,
                    next_focus TEXT,
                    created_at TEXT NOT NULL DEFAULT (datetime('now')),
                    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
                );
            "#,
            kind: MigrationKind::Up,
        },
        Migration {
            version: 4,
            description: "ical subscriptions",
            sql: r#"
                CREATE TABLE IF NOT EXISTS ical_subscriptions (
                    id TEXT PRIMARY KEY,
                    name TEXT NOT NULL,
                    url TEXT NOT NULL,
                    color TEXT DEFAULT '#6b7280',
                    last_synced_at TEXT,
                    error_msg TEXT,
                    created_at TEXT NOT NULL DEFAULT (datetime('now')),
                    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
                );

                CREATE TABLE IF NOT EXISTS ical_events (
                    id TEXT PRIMARY KEY,
                    subscription_id TEXT NOT NULL REFERENCES ical_subscriptions(id) ON DELETE CASCADE,
                    uid TEXT NOT NULL,
                    summary TEXT NOT NULL,
                    description TEXT,
                    location TEXT,
                    start_time TEXT NOT NULL,
                    end_time TEXT NOT NULL,
                    all_day INTEGER NOT NULL DEFAULT 0,
                    created_at TEXT NOT NULL DEFAULT (datetime('now')),
                    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
                    UNIQUE(subscription_id, uid)
                );
            "#,
            kind: MigrationKind::Up,
        },
        Migration {
            version: 5,
            description: "sync log for offline-first mutation queue",
            sql: r#"
                CREATE TABLE IF NOT EXISTS sync_log (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    entity_type TEXT NOT NULL,
                    entity_id TEXT NOT NULL,
                    action TEXT NOT NULL CHECK (action IN ('create', 'update', 'delete')),
                    payload TEXT NOT NULL DEFAULT '{}',
                    synced INTEGER NOT NULL DEFAULT 0,
                    created_at TEXT NOT NULL DEFAULT (datetime('now'))
                );
                CREATE INDEX IF NOT EXISTS idx_sync_pending ON sync_log(synced) WHERE synced = 0;

                CREATE TABLE IF NOT EXISTS sync_state (
                    key TEXT PRIMARY KEY,
                    value TEXT NOT NULL,
                    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
                );
            "#,
            kind: MigrationKind::Up,
        },
        Migration {
            // The server's task rows (and the sync engine's upsertTask) carry a
            // `roughly_at` column that this local schema never had. Pulling any
            // task therefore threw "no column named roughly_at", which aborted the
            // whole pull and left the desktop DB empty. Add it to match the server
            // and the Capacitor schema (schema.ts) column-for-column.
            version: 6,
            description: "add roughly_at to tasks",
            sql: r#"
                ALTER TABLE tasks ADD COLUMN roughly_at TEXT;
            "#,
            kind: MigrationKind::Up,
        },
        Migration {
            // Per-task hard reminders. Mirrors the server (migration 018) and the
            // Capacitor schema (schema.ts) so a pulled task carrying remind_at
            // doesn't break the sync upsert with "no column named remind_at".
            version: 7,
            description: "add remind_at to tasks",
            sql: r#"
                ALTER TABLE tasks ADD COLUMN remind_at TEXT;
            "#,
            kind: MigrationKind::Up,
        },
    ]
}

/// Run migrations — called during app setup.
pub async fn run_migrations(_app: &AppHandle) -> Result<(), Box<dyn std::error::Error>> {
    // Migrations are handled by tauri-plugin-sql when configured with
    // the preload option in tauri.conf.json. The get_migrations() function
    // is used by the plugin builder in lib.rs.
    Ok(())
}
