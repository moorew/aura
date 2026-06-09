/**
 * End-to-end sync engine tests.
 *
 * These run the REAL exported sync()/pullChanges()/pushOutbox() logic against a
 * REAL in-memory SQLite database (node:sqlite), with the server replaced by a
 * scripted fetch stub. The local schema under test is the actual shipped schema
 * — the Tauri migrations parsed from db.rs, and the Capacitor schema from
 * schema.ts — so a schema/engine column mismatch fails here.
 *
 * Regression coverage for the "app opens but is totally empty" report:
 *   1. A pull on a fresh client must WRITE the server's rows into local SQLite
 *      (caught the missing `roughly_at` column, which made every task upsert
 *      throw and aborted the whole pull).
 *   2. A pull that writes rows must bump syncStore.revision so the already-
 *      mounted UI knows to re-read (caught the "rows arrive but board stays
 *      empty until reload" reactivity gap).
 */
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { makeSqliteDriver, type TestDb } from '../test/sqlite-driver';
import { tauriSchemaSql, LOCAL_SCHEMA_SQL } from '../test/schemas';
import { __setTestDriver } from '$lib/tauri/db';

// Pretend we're the Tauri desktop client (the engine uses isTauri() to pick the
// token key and auth scheme). __TAURI__ just needs to be present.
(globalThis as any).window = globalThis.window ?? globalThis;
(globalThis as any).window.__TAURI__ = { core: { invoke: async () => {} }, event: {} };

// Import AFTER the window stub so platform.ts sees __TAURI__.
const { sync, syncStore } = await import('./sync.svelte');

const SERVER = 'https://sempa.test';

function setupClient(schemaSql: string): TestDb {
    const db = makeSqliteDriver();
    db.raw.exec(schemaSql);
    __setTestDriver(db.asDriver());
    localStorage.setItem('sempa_server_url', SERVER);
    localStorage.setItem('sempa_tauri_token', 'test-token');
    return db;
}

// A server "snapshot" the changes feed will return.
interface Feed {
    tasks?: Record<string, unknown>[];
    objectives?: Record<string, unknown>[];
    plans?: Record<string, unknown>[];
    tags?: Record<string, unknown>[];
    week_reviews?: Record<string, unknown>[];
    deletions?: { entity_type: string; entity_id: string; deleted_at: string }[];
    cursor?: string;
}

/**
 * Stub global.fetch to emulate the backend:
 *  - GET /api/v1/health         → 200
 *  - GET /api/v1/sync/changes   → the provided feed
 *  - POST/PATCH/DELETE          → 200 (records the call for push assertions)
 */
function stubServer(feed: Feed) {
    const calls: { method: string; url: string; body?: unknown }[] = [];
    const handler = vi.fn(async (url: string, init?: RequestInit) => {
        const method = init?.method ?? 'GET';
        calls.push({ method, url, body: init?.body ? JSON.parse(init.body as string) : undefined });
        if (url.includes('/api/v1/health')) {
            return new Response('ok', { status: 200 });
        }
        if (url.includes('/api/v1/sync/changes')) {
            return new Response(JSON.stringify({
                tasks: feed.tasks ?? [],
                objectives: feed.objectives ?? [],
                plans: feed.plans ?? [],
                tags: feed.tags ?? [],
                week_reviews: feed.week_reviews ?? [],
                deletions: feed.deletions ?? [],
                cursor: feed.cursor ?? '2026-06-09 12:00:00',
            }), { status: 200, headers: { 'Content-Type': 'application/json' } });
        }
        return new Response('{}', { status: 200 });
    });
    vi.stubGlobal('fetch', handler);
    return calls;
}

function sampleTask(over: Record<string, unknown> = {}): Record<string, unknown> {
    return {
        id: 'task-1', title: 'Write tests', status: 'planned', position: 0,
        planned_date: '2026-06-09', week_start: '2026-06-08',
        tags: ['focus'], is_customized: false,
        created_at: '2026-06-09 10:00:00', updated_at: '2026-06-09 10:00:00',
        roughly_at: 'morning', scheduled_start: null, scheduled_end: null,
        ...over,
    };
}

const SCHEMAS: [string, () => string][] = [
    ['tauri (db.rs migrations)', tauriSchemaSql],
    ['capacitor (schema.ts)', () => LOCAL_SCHEMA_SQL],
];

describe.each(SCHEMAS)('sync engine on %s schema', (_name, schema) => {
    let db: TestDb;

    beforeEach(() => {
        localStorage.clear();
        db = setupClient(schema());
    });

    afterEach(() => {
        __setTestDriver(null);
        db.close();
        vi.unstubAllGlobals();
    });

    it('initial pull writes server tasks into the local DB', async () => {
        stubServer({ tasks: [sampleTask(), sampleTask({ id: 'task-2', title: 'Ship it' })] });

        await sync();

        const rows = db.select<{ id: string; title: string; roughly_at: string }[]>(
            'SELECT id, title, roughly_at FROM tasks ORDER BY id',
        );
        expect(rows).toHaveLength(2);
        expect(rows[0]).toMatchObject({ id: 'task-1', title: 'Write tests', roughly_at: 'morning' });
        expect(rows[1]).toMatchObject({ id: 'task-2', title: 'Ship it' });
    });

    it('pulls FK-laden data without aborting (objective + parent refs)', async () => {
        // Mirrors real data: tasks reference an objective (weekly_objective_id)
        // and a parent task (parent_task_id). The feed lists tasks before
        // objectives, and a child can appear before its parent — both violate
        // the FK if applied naively, which threw "FOREIGN KEY constraint failed
        // (787)" and aborted the entire pull, leaving the app empty.
        stubServer({
            objectives: [{ id: 'obj-1', week_start: '2026-06-08', title: 'Q2 goal', status: 'active',
                position: 0, created_at: '2026-06-09 10:00:00', updated_at: '2026-06-09 10:00:00' }],
            tasks: [
                // child first, references parent that comes later AND the objective
                sampleTask({ id: 'child', title: 'Subtask', parent_task_id: 'parent',
                    weekly_objective_id: 'obj-1' }),
                sampleTask({ id: 'parent', title: 'Parent task', weekly_objective_id: 'obj-1' }),
            ],
        });

        await sync();

        expect(syncStore.lastError).toBeNull();
        expect(db.select('SELECT * FROM tasks')).toHaveLength(2);
        expect(db.select('SELECT * FROM weekly_objectives')).toHaveLength(1);
        const child = db.select<{ parent_task_id: string; weekly_objective_id: string }[]>(
            "SELECT parent_task_id, weekly_objective_id FROM tasks WHERE id = 'child'",
        );
        expect(child[0]).toMatchObject({ parent_task_id: 'parent', weekly_objective_id: 'obj-1' });
    });

    it('a single FK-violating row is skipped, not fatal — the rest still apply', async () => {
        // A task pointing at an objective that is NOT in this batch (e.g. stale
        // reference) must be skipped while every other task is still written.
        // Previously one bad row threw and aborted the whole pull → empty app.
        stubServer({
            tasks: [
                sampleTask({ id: 'ok-1', title: 'Good A' }),
                sampleTask({ id: 'bad', title: 'Dangling', weekly_objective_id: 'missing-obj' }),
                sampleTask({ id: 'ok-2', title: 'Good B' }),
            ],
        });

        await sync();

        const ids = db.select<{ id: string }[]>('SELECT id FROM tasks ORDER BY id').map(r => r.id);
        expect(ids).toContain('ok-1');
        expect(ids).toContain('ok-2');
        expect(ids).not.toContain('bad');
        // The good rows refreshed the UI even though one row failed.
        expect(syncStore.revision).toBeGreaterThan(0);
        // Cursor NOT advanced (so the skipped row is retried next sync).
        const cur = db.select<{ value: string }[]>("SELECT value FROM sync_state WHERE key = 'changes_cursor'");
        expect(cur.length).toBe(0);
    });

    it('pull writes objectives, plans, tags and week_reviews too', async () => {
        stubServer({
            objectives: [{ id: 'o1', week_start: '2026-06-08', title: 'Goal', status: 'active', position: 0,
                created_at: '2026-06-09 10:00:00', updated_at: '2026-06-09 10:00:00' }],
            plans: [{ id: 'p1', plan_date: '2026-06-09', status: 'active',
                created_at: '2026-06-09 10:00:00', updated_at: '2026-06-09 10:00:00' }],
            tags: [{ id: 't1', name: 'focus', color: '#fff',
                created_at: '2026-06-09 10:00:00', updated_at: '2026-06-09 10:00:00' }],
            week_reviews: [{ id: 'w1', week_start: '2026-06-08', wins: 'shipped',
                created_at: '2026-06-09 10:00:00', updated_at: '2026-06-09 10:00:00' }],
        });

        await sync();

        expect(db.select('SELECT * FROM weekly_objectives')).toHaveLength(1);
        expect(db.select('SELECT * FROM daily_plans')).toHaveLength(1);
        expect(db.select('SELECT * FROM tag_definitions')).toHaveLength(1);
        expect(db.select('SELECT * FROM week_reviews')).toHaveLength(1);
    });

    it('bumps syncStore.revision when a pull applies rows (so the UI re-reads)', async () => {
        stubServer({ tasks: [sampleTask()] });
        const before = syncStore.revision;

        await sync();

        expect(syncStore.revision).toBe(before + 1);
        expect(syncStore.lastError).toBeNull();
    });

    it('does NOT bump revision when the server returns nothing new', async () => {
        stubServer({});
        const before = syncStore.revision;

        await sync();

        expect(syncStore.revision).toBe(before);
    });

    it('advances the cursor so the next pull is incremental', async () => {
        stubServer({ tasks: [sampleTask()], cursor: '2026-06-09 13:30:00' });

        await sync();

        const cur = db.select<{ value: string }[]>(
            "SELECT value FROM sync_state WHERE key = 'changes_cursor'",
        );
        expect(cur[0]?.value).toBe('2026-06-09 13:30:00');
    });

    it('last-write-wins: a stale server row does not clobber a newer local edit', async () => {
        // Local row edited at 11:00; server still has the 10:00 copy.
        db.execute(
            `INSERT INTO tasks (id, title, status, position, created_at, updated_at)
             VALUES ('task-1', 'Local newer', 'planned', 0, '2026-06-09 10:00:00', '2026-06-09 11:00:00')`,
        );
        stubServer({ tasks: [sampleTask({ title: 'Server older', updated_at: '2026-06-09 10:00:00' })] });

        await sync();

        const row = db.select<{ title: string }[]>("SELECT title FROM tasks WHERE id = 'task-1'");
        expect(row[0].title).toBe('Local newer');
    });

    it('applies a tombstone by deleting the local row', async () => {
        db.execute(
            `INSERT INTO tasks (id, title, status, position, created_at, updated_at)
             VALUES ('gone', 'Delete me', 'planned', 0, '2026-06-09 10:00:00', '2026-06-09 10:00:00')`,
        );
        stubServer({ deletions: [{ entity_type: 'task', entity_id: 'gone', deleted_at: '2026-06-09 12:00:00' }] });

        await sync();

        expect(db.select("SELECT * FROM tasks WHERE id = 'gone'")).toHaveLength(0);
        expect(syncStore.revision).toBeGreaterThan(0);
    });

    it('pushes a queued local create to the server before pulling', async () => {
        db.execute(
            `INSERT INTO tasks (id, title, status, position, created_at, updated_at)
             VALUES ('local-1', 'Made offline', 'planned', 0, '2026-06-09 09:00:00', '2026-06-09 09:00:00')`,
        );
        db.execute(
            `INSERT INTO sync_log (entity_type, entity_id, action, payload)
             VALUES ('tasks', 'local-1', 'create', '{"title":"Made offline"}')`,
        );
        const calls = stubServer({});

        await sync();

        const post = calls.find(c => c.method === 'POST' && c.url.includes('/api/v1/tasks'));
        expect(post).toBeTruthy();
        expect((post!.body as any).id).toBe('local-1'); // carries client id (idempotent)
        // Outbox entry marked synced.
        const pending = db.select<{ c: number }[]>('SELECT COUNT(*) c FROM sync_log WHERE synced = 0');
        expect(pending[0].c).toBe(0);
    });
});
