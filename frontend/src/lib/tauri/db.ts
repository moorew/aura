/**
 * Local SQLite database access — the offline-first data layer.
 *
 * All reads/writes hit a local SQLite database instantly, and mutations are
 * queued in the `sync_log` outbox for the sync engine to replay against the
 * server when a connection is available. See $lib/sync.ts for the engine.
 *
 * Two backends sit behind one tiny Driver interface:
 *   - Tauri desktop     → @tauri-apps/plugin-sql (migrations run by the plugin)
 *   - Capacitor Android → @capacitor-community/sqlite (schema applied here)
 * The SQL is identical, so $lib/tauri/local-api.ts is shared verbatim.
 */

import { isTauri, isCapacitor, hasLocalDb } from '$lib/platform';
import { LOCAL_SCHEMA_SQL } from './schema';

interface Driver {
    execute(sql: string, params?: unknown[]): Promise<{ rowsAffected: number; lastInsertId: number }>;
    select<T = unknown[]>(sql: string, params?: unknown[]): Promise<T>;
}

let driver: Driver | null = null;
let driverPromise: Promise<Driver> | null = null;

export { hasLocalDb };

// ── Tauri driver (tauri-plugin-sql) ──────────────────────────────────────────

async function loadTauriDriver(): Promise<Driver> {
    const mod = await import('@tauri-apps/plugin-sql');
    const instance = await mod.default.load('sqlite:sempa.db');
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

    driverPromise = (isTauri() ? loadTauriDriver() : loadCapacitorDriver()).then((d) => {
        driver = d;
        return d;
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
