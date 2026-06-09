/**
 * Offline-first sync engine (shared by Tauri desktop and Capacitor Android).
 *
 * The local SQLite DB is the source of truth for the UI. Every local mutation is
 * also appended to the `sync_log` outbox. This engine reconciles local and
 * server state in two phases whenever the server is reachable:
 *
 *   1. PUSH — replay queued outbox mutations against the REST API, in order.
 *   2. PULL — fetch everything changed on the server since our cursor and apply
 *      it locally (last-write-wins by updated_at), including deletions.
 *
 * It runs on app start, on reconnect (online event / SSE reopen), after a local
 * write, and on a periodic timer. Concurrent runs are coalesced.
 */

import { isTauri } from './tauri/bridge';
import {
    getPendingMutations,
    markMutationsSynced,
    getPendingMutationCount,
    getSyncState,
    setSyncState,
    execute,
    query,
    type PendingMutation,
} from './tauri/db';

const CURSOR_KEY = 'changes_cursor';

// localStorage-backed config (same keys as $lib/api). Read directly to avoid a
// circular import (api → local-api → sync → api).
function getServerUrl(): string {
    return typeof localStorage !== 'undefined' ? (localStorage.getItem('sempa_server_url') ?? '') : '';
}
function getTauriToken(): string {
    return typeof localStorage !== 'undefined' ? (localStorage.getItem('sempa_tauri_token') ?? '') : '';
}
function getNativeToken(): string {
    return typeof localStorage !== 'undefined' ? (localStorage.getItem('sempa_native_token') ?? '') : '';
}

// ── Sync status store (Svelte 5 runes) ──────────────────────────────────────

export interface SyncState {
    online: boolean;
    syncing: boolean;
    pending: number;
    lastSyncedAt: string | null;
    lastError: string | null;
}

function createSyncStore() {
    let online = $state(false);
    let syncing = $state(false);
    let pending = $state(0);
    let lastSyncedAt = $state<string | null>(null);
    let lastError = $state<string | null>(null);
    // Bumped whenever a pull writes ≥1 row to the local DB. The UI reads the
    // local DB once on mount, so without a reactive signal a freshly-pulled
    // dataset would stay invisible until a manual reload. Pages/layout watch
    // this to re-read after the initial (and every subsequent) pull lands.
    let revision = $state(0);

    return {
        get online() { return online; },
        get syncing() { return syncing; },
        get pending() { return pending; },
        get lastSyncedAt() { return lastSyncedAt; },
        get lastError() { return lastError; },
        get revision() { return revision; },
        _set(p: Partial<SyncState>) {
            if (p.online !== undefined) online = p.online;
            if (p.syncing !== undefined) syncing = p.syncing;
            if (p.pending !== undefined) pending = p.pending;
            if (p.lastSyncedAt !== undefined) lastSyncedAt = p.lastSyncedAt;
            if (p.lastError !== undefined) lastError = p.lastError;
        },
        _bumpRevision() { revision += 1; },
    };
}

export const syncStore = createSyncStore();

// ── HTTP plumbing (talks to the server directly, bypassing the local-first api) ──

function authHeader(): Record<string, string> {
    const token = isTauri() ? getTauriToken() : getNativeToken();
    return token ? { Authorization: `Bearer ${token}` } : {};
}

async function serverFetch(path: string, init?: RequestInit): Promise<Response> {
    const base = getServerUrl();
    const token = isTauri() ? getTauriToken() : getNativeToken();
    return fetch(`${base}${path}`, {
        ...init,
        headers: { 'Content-Type': 'application/json', ...authHeader(), ...(init?.headers ?? {}) },
        credentials: token ? 'omit' : 'include',
    });
}

// ── Server change payload (mirror of db.SyncChanges) ─────────────────────────

interface Tombstone { entity_type: string; entity_id: string; deleted_at: string }
interface ServerChanges {
    tasks: Record<string, unknown>[];
    objectives: Record<string, unknown>[];
    plans: Record<string, unknown>[];
    tags: Record<string, unknown>[];
    week_reviews: Record<string, unknown>[];
    deletions: Tombstone[];
    cursor: string;
}

// ── Push: replay the outbox ──────────────────────────────────────────────────

// Maps an outbox entry to its REST call. Returns true on success (entry will be
// marked synced), false to leave it queued for the next attempt.
async function replay(m: PendingMutation): Promise<boolean> {
    const payload = m.payload ? JSON.parse(m.payload) : {};
    const path = restPath(m.entity_type);
    if (!path) return true; // unknown entity → drop, don't wedge the queue

    try {
        let res: Response;
        if (m.action === 'create') {
            // Carry the client id so the server row shares it (idempotent).
            res = await serverFetch(path, { method: 'POST', body: JSON.stringify({ ...payload, id: m.entity_id }) });
        } else if (m.action === 'update') {
            res = await serverFetch(`${path}/${encodeURIComponent(m.entity_id)}`, {
                method: 'PATCH', body: JSON.stringify(payload),
            });
        } else {
            res = await serverFetch(`${path}/${encodeURIComponent(m.entity_id)}`, { method: 'DELETE' });
        }

        // 2xx = applied. 404 on update/delete = already gone server-side; treat
        // as done so a deleted-elsewhere row doesn't wedge the queue forever.
        if (res.ok) return true;
        if (res.status === 404 && m.action !== 'create') return true;
        return false;
    } catch {
        return false; // network error — keep queued
    }
}

// Plans use PUT /plans/{date} keyed by date, and reviews PUT /weeks/{ws}/review;
// both are handled by their own upsert paths rather than the generic replay.
function restPath(entityType: string): string | null {
    switch (entityType) {
        case 'tasks': return '/api/v1/tasks';
        case 'objectives': return '/api/v1/objectives';
        case 'tags': return '/api/v1/tags';
        case 'plans': return '/api/v1/plans';        // create/update both PUT /{date}
        case 'week_reviews': return '/api/v1/weeks';  // PUT /{ws}/review
        default: return null;
    }
}

async function pushOutbox(): Promise<void> {
    const pending = await getPendingMutations();
    const done: number[] = [];
    for (const m of pending) {
        let ok: boolean;
        if (m.entity_type === 'plans') {
            ok = await replayPlan(m);
        } else if (m.entity_type === 'week_reviews') {
            ok = await replayWeekReview(m);
        } else {
            ok = await replay(m);
        }
        if (!ok) break; // preserve order — stop at first failure, retry next cycle
        done.push(m.id);
    }
    await markMutationsSynced(done);
}

// Plans are upserted by date (PUT /plans/{date}); the payload carries plan_date.
async function replayPlan(m: PendingMutation): Promise<boolean> {
    const payload = m.payload ? JSON.parse(m.payload) : {};
    const date = payload.plan_date ?? m.entity_id;
    try {
        const res = await serverFetch(`/api/v1/plans/${encodeURIComponent(date)}`, {
            method: 'PUT', body: JSON.stringify(payload),
        });
        return res.ok;
    } catch { return false; }
}

// Week reviews are upserted by week_start (PUT /weeks/{ws}/review).
async function replayWeekReview(m: PendingMutation): Promise<boolean> {
    const payload = m.payload ? JSON.parse(m.payload) : {};
    const ws = payload.week_start ?? m.entity_id;
    try {
        const res = await serverFetch(`/api/v1/weeks/${encodeURIComponent(ws)}/review`, {
            method: 'PUT', body: JSON.stringify(payload),
        });
        return res.ok;
    } catch { return false; }
}

// ── Pull: apply server changes locally ───────────────────────────────────────

async function pullChanges(): Promise<void> {
    const since = (await getSyncState(CURSOR_KEY)) ?? '';
    const res = await serverFetch(`/api/v1/sync/changes?since=${encodeURIComponent(since)}`);
    if (!res.ok) throw new Error(`pull failed: ${res.status}`);
    const changes: ServerChanges = await res.json();

    let applied = 0;
    for (const t of changes.tasks) applied += (await upsertTask(t)) ? 1 : 0;
    for (const o of changes.objectives) applied += (await upsertObjective(o)) ? 1 : 0;
    for (const p of changes.plans) applied += (await upsertPlan(p)) ? 1 : 0;
    for (const tag of changes.tags) applied += (await upsertTag(tag)) ? 1 : 0;
    for (const r of changes.week_reviews) applied += (await upsertWeekReview(r)) ? 1 : 0;
    for (const d of changes.deletions) applied += (await applyDeletion(d)) ? 1 : 0;

    if (changes.cursor) await setSyncState(CURSOR_KEY, changes.cursor);

    // Anything actually changed locally → tell the UI to re-read. Without this,
    // the first pull silently fills SQLite but the already-mounted pages keep
    // showing their initial (empty) snapshot until a manual reload.
    if (applied > 0) syncStore._bumpRevision();
}

// Last-write-wins guard: returns true only when the incoming row should be
// written, i.e. there's no local row or the incoming updated_at is strictly
// newer. This protects a not-yet-pushed local edit from being clobbered by a
// stale server copy (the local edit wins on the next push).
//
// keyCol matters: tasks/objectives/tags share their id with the server (client
// ids), but daily_plans and week_reviews are upserted server-side by a natural
// key (plan_date / week_start) and get a *different* server id, so they must be
// matched on that natural key — both here and in the ON CONFLICT target below.
async function lww(table: string, row: Record<string, unknown>, keyCol = 'id'): Promise<boolean> {
    const key = row[keyCol] as string;
    const existing = await query<{ updated_at: string }[]>(
        `SELECT updated_at FROM ${table} WHERE ${keyCol} = ?`, [key],
    );
    if (existing.length > 0) {
        const localTs = existing[0].updated_at ?? '';
        const remoteTs = (row.updated_at as string) ?? '';
        if (remoteTs <= localTs) return false; // local is same-or-newer → keep it
    }
    return true;
}

async function upsertTask(t: Record<string, unknown>): Promise<boolean> {
    if (!(await lww('tasks', t))) return false;
    await execute(
        `INSERT INTO tasks (id, title, description, planned_date, week_start, status, position,
            time_estimate_minutes, time_actual_minutes, parent_task_id, weekly_objective_id,
            source, source_id, source_url, source_metadata, completed_at, archived_at,
            created_at, updated_at, tags, recurrence_rule, recurrence_origin_id, is_customized,
            scheduled_start, scheduled_end, roughly_at)
         VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
         ON CONFLICT(id) DO UPDATE SET
            title=excluded.title, description=excluded.description, planned_date=excluded.planned_date,
            week_start=excluded.week_start, status=excluded.status, position=excluded.position,
            time_estimate_minutes=excluded.time_estimate_minutes, time_actual_minutes=excluded.time_actual_minutes,
            parent_task_id=excluded.parent_task_id, weekly_objective_id=excluded.weekly_objective_id,
            source=excluded.source, source_id=excluded.source_id, source_url=excluded.source_url,
            source_metadata=excluded.source_metadata, completed_at=excluded.completed_at,
            archived_at=excluded.archived_at, updated_at=excluded.updated_at, tags=excluded.tags,
            recurrence_rule=excluded.recurrence_rule, recurrence_origin_id=excluded.recurrence_origin_id,
            is_customized=excluded.is_customized, scheduled_start=excluded.scheduled_start,
            scheduled_end=excluded.scheduled_end, roughly_at=excluded.roughly_at`,
        [
            t.id, t.title, t.description ?? null, t.planned_date ?? null, t.week_start ?? null,
            t.status, t.position ?? 0, t.time_estimate_minutes ?? null, t.time_actual_minutes ?? null,
            t.parent_task_id ?? null, t.weekly_objective_id ?? null, t.source ?? null, t.source_id ?? null,
            t.source_url ?? null, t.source_metadata ?? null, t.completed_at ?? null, t.archived_at ?? null,
            t.created_at ?? null, t.updated_at ?? null, JSON.stringify(t.tags ?? []),
            t.recurrence_rule ?? null, t.recurrence_origin_id ?? null, t.is_customized ? 1 : 0,
            t.scheduled_start ?? null, t.scheduled_end ?? null, t.roughly_at ?? null,
        ],
    );
    return true;
}

async function upsertObjective(o: Record<string, unknown>): Promise<boolean> {
    if (!(await lww('weekly_objectives', o))) return false;
    await execute(
        `INSERT INTO weekly_objectives (id, week_start, title, description, status, position, created_at, updated_at)
         VALUES (?,?,?,?,?,?,?,?)
         ON CONFLICT(id) DO UPDATE SET
            week_start=excluded.week_start, title=excluded.title, description=excluded.description,
            status=excluded.status, position=excluded.position, updated_at=excluded.updated_at`,
        [o.id, o.week_start, o.title, o.description ?? null, o.status ?? 'active', o.position ?? 0,
         o.created_at ?? null, o.updated_at ?? null],
    );
    return true;
}

async function upsertPlan(p: Record<string, unknown>): Promise<boolean> {
    if (!(await lww('daily_plans', p, 'plan_date'))) return false;
    // Conflict on plan_date (the natural key): the server row's id differs from
    // any local id, so id-based upsert would hit the UNIQUE(plan_date) constraint.
    await execute(
        `INSERT INTO daily_plans (id, plan_date, status, intention, reflection, wins, shutdown_at, created_at, updated_at)
         VALUES (?,?,?,?,?,?,?,?,?)
         ON CONFLICT(plan_date) DO UPDATE SET
            id=excluded.id, status=excluded.status, intention=excluded.intention,
            reflection=excluded.reflection, wins=excluded.wins, shutdown_at=excluded.shutdown_at,
            updated_at=excluded.updated_at`,
        [p.id, p.plan_date, p.status ?? 'pending', p.intention ?? null, p.reflection ?? null,
         p.wins ?? null, p.shutdown_at ?? null, p.created_at ?? null, p.updated_at ?? null],
    );
    return true;
}

async function upsertTag(tag: Record<string, unknown>): Promise<boolean> {
    if (!(await lww('tag_definitions', tag))) return false;
    await execute(
        `INSERT INTO tag_definitions (id, name, color, created_at, updated_at)
         VALUES (?,?,?,?,?)
         ON CONFLICT(id) DO UPDATE SET
            name=excluded.name, color=excluded.color, updated_at=excluded.updated_at`,
        [tag.id, tag.name, tag.color ?? '#6366f1', tag.created_at ?? null, tag.updated_at ?? null],
    );
    return true;
}

async function upsertWeekReview(r: Record<string, unknown>): Promise<boolean> {
    if (!(await lww('week_reviews', r, 'week_start'))) return false;
    // Conflict on week_start (the natural key) — same reasoning as plans.
    await execute(
        `INSERT INTO week_reviews (id, week_start, wins, challenges, next_focus, created_at, updated_at)
         VALUES (?,?,?,?,?,?,?)
         ON CONFLICT(week_start) DO UPDATE SET
            id=excluded.id, wins=excluded.wins, challenges=excluded.challenges,
            next_focus=excluded.next_focus, updated_at=excluded.updated_at`,
        [r.id, r.week_start, r.wins ?? null, r.challenges ?? null, r.next_focus ?? null,
         r.created_at ?? null, r.updated_at ?? null],
    );
    return true;
}

const TOMBSTONE_TABLE: Record<string, string> = {
    task: 'tasks',
    objective: 'weekly_objectives',
    plan: 'daily_plans',
    tag: 'tag_definitions',
    week_review: 'week_reviews',
};

async function applyDeletion(d: Tombstone): Promise<boolean> {
    const table = TOMBSTONE_TABLE[d.entity_type];
    if (!table) return false;
    const res = await execute(`DELETE FROM ${table} WHERE id = ?`, [d.entity_id]);
    return (res?.rowsAffected ?? 0) > 0;
}

// ── Orchestration ────────────────────────────────────────────────────────────

let running = false;
let queued = false;

async function reachable(): Promise<boolean> {
    if (!getServerUrl()) return false;
    try {
        const res = await serverFetch('/api/v1/health');
        return res.ok;
    } catch {
        return false;
    }
}

/**
 * Run one full sync cycle (push then pull). Safe to call often: concurrent calls
 * are coalesced into a single trailing run.
 */
export async function sync(): Promise<void> {
    if (running) { queued = true; return; }
    running = true;
    try {
        syncStore._set({ pending: await getPendingMutationCount() });

        const online = await reachable();
        syncStore._set({ online });
        if (!online) return;

        syncStore._set({ syncing: true, lastError: null });
        await pushOutbox();
        await pullChanges();
        syncStore._set({
            lastSyncedAt: new Date().toISOString(),
            pending: await getPendingMutationCount(),
        });
    } catch (e) {
        syncStore._set({ lastError: e instanceof Error ? e.message : String(e) });
    } finally {
        syncStore._set({ syncing: false });
        running = false;
        if (queued) { queued = false; void sync(); }
    }
}

let interval: ReturnType<typeof setInterval> | null = null;
let onlineHandler: (() => void) | null = null;

/** Start background sync: initial cycle, on reconnect, and every 30s. */
export function startSync(): void {
    if (interval) return; // already started
    void sync();
    interval = setInterval(() => void sync(), 30_000);
    onlineHandler = () => void sync();
    if (typeof window !== 'undefined') window.addEventListener('online', onlineHandler);
}

export function stopSync(): void {
    if (interval) { clearInterval(interval); interval = null; }
    if (onlineHandler && typeof window !== 'undefined') {
        window.removeEventListener('online', onlineHandler);
        onlineHandler = null;
    }
}

/** Nudge a sync shortly after a local write so changes propagate promptly. */
let flushTimer: ReturnType<typeof setTimeout> | null = null;
export function flushSoon(): void {
    if (flushTimer) return;
    flushTimer = setTimeout(() => { flushTimer = null; void sync(); }, 800);
}
