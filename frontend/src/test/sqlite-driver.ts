/**
 * A Driver (see $lib/tauri/db.ts) backed by Node 22+/24's built-in `node:sqlite`.
 *
 * This lets the real sync engine and local-api run against a genuine in-memory
 * SQLite database in unit tests — no Tauri/Capacitor runtime, no native deps.
 * The SQL executed is exactly the SQL that runs in production, so these tests
 * exercise the actual upsert/pull/outbox logic rather than a reimplementation.
 */
import { DatabaseSync } from 'node:sqlite';
import type { Driver } from '$lib/tauri/db';

// node:sqlite is synchronous, so the test driver exposes sync execute/select
// (handy for terse assertions). The production Driver interface is async; the
// engine just awaits the (non-promise) return, which is fine. asDriver() casts
// it for __setTestDriver without leaking the mismatch into the test body.
export interface TestDb {
    raw: DatabaseSync;
    execute(sql: string, params?: unknown[]): { rowsAffected: number; lastInsertId: number };
    select<T = unknown[]>(sql: string, params?: unknown[]): T;
    close(): void;
    asDriver(): Driver;
}

export function makeSqliteDriver(): TestDb {
    const db = new DatabaseSync(':memory:');
    db.exec('PRAGMA foreign_keys = ON;');

    const self: TestDb = {
        raw: db,
        execute(sql: string, params: unknown[] = []) {
            const info = db.prepare(sql).run(...(params as never[]));
            return {
                rowsAffected: Number(info.changes ?? 0),
                lastInsertId: Number(info.lastInsertRowid ?? 0),
            };
        },
        select<T = unknown[]>(sql: string, params: unknown[] = []): T {
            return db.prepare(sql).all(...(params as never[])) as T;
        },
        close() {
            db.close();
        },
        asDriver() {
            return self as unknown as Driver;
        },
    };
    return self;
}
