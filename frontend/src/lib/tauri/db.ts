/**
 * Local SQLite database access — the offline-first data layer.
 *
 * All reads/writes hit a local SQLite database instantly, and mutations are
 * queued in the `sync_log` outbox for the sync engine to replay against the
 * server when a connection is available. See $lib/sync.svelte.ts for the engine.
 *
 * Two backends sit behind one tiny Driver interface:
 *   - Tauri desktop     → @tauri-apps/plugin-sql (migrations run by the plugin)
 *   - Capacitor Android → @capacitor-community/sqlite (schema applied here)
 * The SQL is identical, so $lib/tauri/local-api.ts is shared verbatim.
 */

import { isTauri, isCapacitor, hasLocalDb } from '$lib/platform';
import { LOCAL_SCHEMA_SQL } from './schema';
// Statically imported (NOT dynamic import()) on purpose. A dynamic import here
// loads a separate JS chunk at runtime, which in the Tauri webview can fail with
// "Failed to fetch dynamically imported module" — and since this is the FIRST
// thing every local DB read/write touches, that failure breaks the entire
// offline-first path (tasks silently fail to save). The module is tiny and only
// performs IPC when its functions are *called*, so importing it eagerly is safe
// on every platform (desktop, Android, plain web).
import Database from '@tauri-apps/plugin-sql';

interface Driver {
    execute(sql: string, params?: unknown[]): Promise<{ rowsAffected: number; lastInsertId: number }>;
    select<T = unknown[]>(sql: string, params?: unknown[]): Promise<T>;
}

let driver: Driver | null = null;
let driverPromise: Promise<Driver> | null = null;

export { hasLocalDb };
export type { Driver };

/**
 * Test-only: inject a Driver (e.g. backed by node:sqlite) so the sync engine and
 * local-api can run against a real in-memory SQLite database in unit tests,
 * without a Tauri/Capacitor runtime. Pass null to reset. Not used in production.
 */
export function __setTestDriver(d: Driver | null): void {
    driver = d;
    driverPromise = null;
}

// ── Tauri driver (tauri-plugin-sql) ──────────────────────────────────────────

async function loadTauriDriver(): Promise<Driver> {
    const instance = await Database.load('sqlite:sempa.db');
    return {
        execute: (sql, params) => instance.execute(sql, params),
        select: (sql, params) => instance.select(sql, params),
    } as Driver;
}

// ── Capacitor driver (@capacitor-community/sqlite) ───────────────────────────
//
// Requires the @capacitor-community/sqlite native plugin in the Android build.
// The plugin has no migration runner like tauri-plugin-sql, so we apply the
// shared schema once on open. LOCAL_SCHEMA_SQL is idempotent (CREATE TABLE IF
// NOT EXISTS) so re-applying on every launch is safe and cheap.

// Columns added AFTER a table's first shipped shape. `CREATE TABLE IF NOT EXISTS`
// cannot add columns to a table that already exists, and Capacitor has no
// migration runner — so an Android install created before one of these columns
// shipped is missing it, and any query touching it (e.g. saving a task with a
// reminder → `UPDATE tasks SET remind_at = ?`) throws "no such column" and the
// save silently fails. We reconcile by ADD COLUMN-ing whatever is missing. Each
// entry must be nullable or carry a constant DEFAULT (ALTER ADD COLUMN can't add
// a bare NOT NULL), and must mirror schema.ts. Re-running is a no-op (we skip
// columns already present), so this is safe on every launch.
const COLUMN_RECONCILE: Record<string, Array<[string, string]>> = {
    tasks: [
        ['time_estimate_minutes', 'INTEGER'],
        ['time_actual_minutes', 'INTEGER'],
        ['parent_task_id', 'TEXT'],
        ['weekly_objective_id', 'TEXT'],
        ['source', "TEXT DEFAULT 'manual'"],
        ['source_id', 'TEXT'],
        ['source_url', 'TEXT'],
        ['source_metadata', 'TEXT'],
        ['completed_at', 'TEXT'],
        ['archived_at', 'TEXT'],
        ['tags', "TEXT DEFAULT '[]'"],
        ['recurrence_rule', 'TEXT'],
        ['recurrence_origin_id', 'TEXT'],
        ['is_customized', 'INTEGER NOT NULL DEFAULT 0'],
        ['scheduled_start', 'TEXT'],
        ['scheduled_end', 'TEXT'],
        ['roughly_at', 'TEXT'],
        ['remind_at', 'TEXT'],
    ],
    daily_plans: [
        ['intention', 'TEXT'],
        ['reflection', 'TEXT'],
        ['wins', 'TEXT'],
        ['shutdown_at', 'TEXT'],
    ],
};

interface CapConn {
    query(sql: string, params?: unknown[]): Promise<{ values?: unknown[] }>;
    execute(sql: string): Promise<unknown>;
}

async function reconcileColumns(conn: CapConn): Promise<void> {
    for (const [table, cols] of Object.entries(COLUMN_RECONCILE)) {
        let existing: Set<string>;
        try {
            const info = await conn.query(`PRAGMA table_info(${table})`);
            existing = new Set((info.values ?? []).map((r) => (r as { name: string }).name));
        } catch {
            continue; // table not present yet (created fresh by schema) — nothing to patch
        }
        for (const [name, type] of cols) {
            if (existing.has(name)) continue;
            try {
                await conn.execute(`ALTER TABLE ${table} ADD COLUMN ${name} ${type}`);
            } catch {
                /* raced / already added — ignore */
            }
        }
    }
}

async function loadCapacitorDriver(): Promise<Driver> {
    const mod = await import('@capacitor-community/sqlite');
    const sqlite = new mod.SQLiteConnection(mod.CapacitorSQLite);
    const dbName = 'sempa';

    const isConn = (await sqlite.isConnection(dbName, false)).result;
    const conn = isConn
        ? await sqlite.retrieveConnection(dbName, false)
        : await sqlite.createConnection(dbName, false, 'no-encryption', 1, false);

    await conn.open();
    await conn.execute(LOCAL_SCHEMA_SQL);
    // Patch in any columns missing on installs that predate them (see above).
    await reconcileColumns(conn as unknown as CapConn);

    return {
        execute: async (sql, params) => {
            const res = await conn.run(sql, (params as never[]) ?? [], false);
            return {
                rowsAffected: res.changes?.changes ?? 0,
                lastInsertId: res.changes?.lastId ?? 0,
            };
        },
        select: async <T>(sql: string, params?: unknown[]) => {
            const res = await conn.query(sql, (params as never[]) ?? []);
            return (res.values ?? []) as T;
        },
    } as Driver;
}

async function getDriver(): Promise<Driver> {
    if (driver) return driver;
    if (driverPromise) return driverPromise;
    if (!hasLocalDb()) throw new Error('No local database on this platform');

    driverPromise = (isTauri() ? loadTauriDriver() : loadCapacitorDriver())
        .then((d) => {
            driver = d;
            return d;
        })
        .catch((e) => {
            // Don't cache a rejected promise — a transient open failure would
            // otherwise wedge the local DB forever. Surface a clear message so a
            // driver problem never masquerades as a generic "Failed to fetch".
            driverPromise = null;
            const detail = e instanceof Error ? e.message : String(e);
            throw new Error(`Local database unavailable: ${detail}`);
        });
    return driverPromise;
}

// ── Sync log (outbox) helpers ────────────────────────────────────────────────

export interface PendingMutation {
    id: number;
    entity_type: string;
    entity_id: string;
    action: 'create' | 'update' | 'delete';
    payload: string; // JSON
}

export async function logMutation(
    entityType: string,
    entityId: string,
    action: 'create' | 'update' | 'delete',
    payload: Record<string, unknown>,
): Promise<void> {
    const d = await getDriver();
    await d.execute(
        `INSERT INTO sync_log (entity_type, entity_id, action, payload) VALUES (?, ?, ?, ?)`,
        [entityType, entityId, action, JSON.stringify(payload)],
    );
}

/** All unsynced mutations, oldest first — the order they must be replayed in. */
export async function getPendingMutations(): Promise<PendingMutation[]> {
    const d = await getDriver();
    return d.select<PendingMutation[]>(
        `SELECT id, entity_type, entity_id, action, payload
         FROM sync_log WHERE synced = 0 ORDER BY id ASC`,
    );
}

export async function getPendingMutationCount(): Promise<number> {
    const d = await getDriver();
    const rows = await d.select<{ count: number }[]>(
        `SELECT COUNT(*) as count FROM sync_log WHERE synced = 0`,
    );
    return rows[0]?.count ?? 0;
}

export async function markMutationsSynced(ids: number[]): Promise<void> {
    if (ids.length === 0) return;
    const d = await getDriver();
    const placeholders = ids.map(() => '?').join(',');
    await d.execute(`UPDATE sync_log SET synced = 1 WHERE id IN (${placeholders})`, ids);
}

// ── Sync cursor (sync_state key/value) ───────────────────────────────────────

export async function getSyncState(key: string): Promise<string | null> {
    const d = await getDriver();
    const rows = await d.select<{ value: string }[]>(
        `SELECT value FROM sync_state WHERE key = ?`, [key],
    );
    return rows[0]?.value ?? null;
}

export async function setSyncState(key: string, value: string): Promise<void> {
    const d = await getDriver();
    await d.execute(
        `INSERT INTO sync_state (key, value, updated_at) VALUES (?, ?, datetime('now'))
         ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now')`,
        [key, value],
    );
}

// ── Generic query helpers ───────────────────────────────────────────────────

export async function query<T = unknown[]>(sql: string, params?: unknown[]): Promise<T> {
    const d = await getDriver();
    return d.select<T>(sql, params);
}

export async function execute(
    sql: string,
    params?: unknown[],
): Promise<{ rowsAffected: number; lastInsertId: number }> {
    const d = await getDriver();
    return d.execute(sql, params);
}
