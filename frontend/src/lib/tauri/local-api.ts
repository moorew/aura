/**
 * Local SQLite-backed API — the offline-first data layer for Tauri desktop and
 * Capacitor Android. Mirrors the HTTP api surface from $lib/api but reads/writes
 * local SQLite. Every write also appends to the sync_log outbox (logMutation)
 * and nudges the sync engine (flushSoon) so changes reach the server when it's
 * reachable. See $lib/sync.svelte.ts.
 */

import { query, execute, logMutation } from './db';
import { flushSoon } from '$lib/sync.svelte';
import type {
    Task, CreateTaskInput, UpdateTaskInput,
    Objective, CreateObjectiveInput, UpdateObjectiveInput,
    DailyPlan, UpsertPlanInput,
    PomodoroSession, TagDefinition, WeekReview,
} from '$lib/types';
import { weekStart as computeWeekStart } from '$lib/utils';

function uuid(): string {
    return crypto.randomUUID();
}

function now(): string {
    return new Date().toISOString().replace('T', ' ').replace('Z', '').slice(0, 19);
}

function parseTaskRow(row: Record<string, unknown>): Task {
    return {
        ...row,
        tags: typeof row.tags === 'string' ? JSON.parse(row.tags || '[]') : (row.tags ?? []),
        is_customized: Boolean(row.is_customized),
    } as Task;
}

export const localApi = {
    setup: {
        status: async () => ({ done: true }),
        complete: async () => ({ done: true }),
    },

    auth: {
        config: async () => ({ google_enabled: false, password_enabled: false }),
        me: async () => ({ authenticated: true, auth_enabled: false, google_enabled: false, email: 'local', username: 'local' }),
        login: async () => ({ status: 'ok' }),
        logout: async () => {},
        nativeFinalize: async () => ({ status: 'ok' }),
    },

    tasks: {
        listByDate: async (date: string): Promise<Task[]> => {
            const rows = await query<Record<string, unknown>[]>(
                `SELECT * FROM tasks WHERE planned_date = ? AND status != 'cancelled' ORDER BY position ASC`,
                [date],
            );
            return rows.map(parseTaskRow);
        },
        listByWeek: async (ws: string): Promise<Task[]> => {
            const rows = await query<Record<string, unknown>[]>(
                `SELECT * FROM tasks WHERE week_start = ? OR (planned_date >= ? AND planned_date < date(?, '+7 days')) ORDER BY position ASC`,
                [ws, ws, ws],
            );
            return rows.map(parseTaskRow);
        },
        listBacklog: async (): Promise<Task[]> => {
            const rows = await query<Record<string, unknown>[]>(
                `SELECT * FROM tasks WHERE status = 'backlog' ORDER BY position ASC`,
            );
            return rows.map(parseTaskRow);
        },
        listByRecurrenceOrigin: async (originId: string): Promise<Task[]> => {
            const rows = await query<Record<string, unknown>[]>(
                `SELECT * FROM tasks WHERE recurrence_origin_id = ? ORDER BY planned_date ASC`,
                [originId],
            );
            return rows.map(parseTaskRow);
        },
        listBySource: async (source: string): Promise<Task[]> => {
            const rows = await query<Record<string, unknown>[]>(
                `SELECT * FROM tasks WHERE source = ? ORDER BY position ASC`,
                [source],
            );
            return rows.map(parseTaskRow);
        },
        listByParent: async (parentId: string): Promise<Task[]> => {
            const rows = await query<Record<string, unknown>[]>(
                `SELECT * FROM tasks WHERE parent_task_id = ? ORDER BY position ASC`,
                [parentId],
            );
            return rows.map(parseTaskRow);
        },
        get: async (id: string): Promise<Task> => {
            const rows = await query<Record<string, unknown>[]>(
                `SELECT * FROM tasks WHERE id = ?`, [id],
            );
            if (rows.length === 0) throw new Error('Task not found');
            return parseTaskRow(rows[0]);
        },
        create: async (input: CreateTaskInput): Promise<Task> => {
            const id = uuid();
            const ts = now();
            const ws = input.week_start ?? (input.planned_date ? computeWeekStart(input.planned_date) : null);
            await execute(
                `INSERT INTO tasks (id, title, description, planned_date, week_start, status, position,
                 time_estimate_minutes, weekly_objective_id, parent_task_id, tags,
                 recurrence_rule, scheduled_start, scheduled_end, created_at, updated_at)
                 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
                [
                    id, input.title, input.description ?? null, input.planned_date ?? null,
                    ws, input.status ?? 'planned', input.position ?? 0,
                    input.time_estimate_minutes ?? null, input.weekly_objective_id ?? null,
                    input.parent_task_id ?? null, JSON.stringify(input.tags ?? []),
                    input.recurrence_rule ?? null, input.scheduled_start ?? null,
                    input.scheduled_end ?? null, ts, ts,
                ],
            );
            // Log the full created row (not just input) so server-side computed
            // fields like week_start/status propagate verbatim on replay.
            const created = await localApi.tasks.get(id);
            await logMutation('tasks', id, 'create', created as unknown as Record<string, unknown>);
            flushSoon();
            return created;
        },
        update: async (id: string, patch: UpdateTaskInput): Promise<Task> => {
            const sets: string[] = [];
            const vals: unknown[] = [];
            const entries = Object.entries(patch).filter(([, v]) => v !== undefined);
            for (const [key, val] of entries) {
                if (key === 'tags') {
                    sets.push('tags = ?');
                    vals.push(JSON.stringify(val));
                } else {
                    sets.push(`${key} = ?`);
                    vals.push(val);
                }
            }
            if (sets.length === 0) return localApi.tasks.get(id);
            sets.push('updated_at = ?');
            vals.push(now());
            vals.push(id);
            await execute(`UPDATE tasks SET ${sets.join(', ')} WHERE id = ?`, vals);
            await logMutation('tasks', id, 'update', patch as unknown as Record<string, unknown>);
            flushSoon();
            return localApi.tasks.get(id);
        },
        delete: async (id: string): Promise<void> => {
            await execute(`DELETE FROM tasks WHERE id = ?`, [id]);
            await logMutation('tasks', id, 'delete', {});
            flushSoon();
        },
    },

    devices: {
        register: async () => ({}),
        unregister: async () => {},
    },

    // Attachments require server-side blob storage; in offline desktop mode they
    // are unavailable. Stubs keep the UI from crashing.
    attachments: {
        listForTask: async () => [],
        listForObjective: async () => [],
        uploadToTask: async () => { throw new Error('Attachments need a server connection'); },
        uploadToObjective: async () => { throw new Error('Attachments need a server connection'); },
        delete: async () => {},
        downloadUrl: () => '',
    },

    // Backups are a server-side feature; unavailable in offline desktop mode.
    backup: {
        getSettings: async () => { throw new Error('Backups need a server connection'); },
        updateSettings: async () => { throw new Error('Backups need a server connection'); },
        runs: async () => [],
        run: async () => { throw new Error('Backups need a server connection'); },
        test: async () => ({ ok: false, error: 'Backups need a server connection' }),
        downloadUrl: () => '',
        restore: async () => { throw new Error('Backups need a server connection'); },
        driveAuthUrl: () => '',
        driveStatus: async () => ({ connected: false }),
        driveDisconnect: async () => {},
    },

    objectives: {
        listByWeek: async (ws: string): Promise<Objective[]> => {
            return query<Objective[]>(
                `SELECT * FROM weekly_objectives WHERE week_start = ? ORDER BY position ASC`, [ws],
            );
        },
        get: async (id: string): Promise<Objective> => {
            const rows = await query<Objective[]>(`SELECT * FROM weekly_objectives WHERE id = ?`, [id]);
            if (rows.length === 0) throw new Error('Objective not found');
            return rows[0];
        },
        create: async (input: CreateObjectiveInput): Promise<Objective> => {
            const id = uuid();
            const ts = now();
            await execute(
                `INSERT INTO weekly_objectives (id, week_start, title, description, status, position, created_at, updated_at)
                 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
                [id, input.week_start, input.title, input.description ?? null, 'active', input.position ?? 0, ts, ts],
            );
            const created = await localApi.objectives.get(id);
            await logMutation('objectives', id, 'create', created as unknown as Record<string, unknown>);
            flushSoon();
            return created;
        },
        update: async (id: string, patch: UpdateObjectiveInput): Promise<Objective> => {
            const sets: string[] = [];
            const vals: unknown[] = [];
            for (const [key, val] of Object.entries(patch).filter(([, v]) => v !== undefined)) {
                sets.push(`${key} = ?`);
                vals.push(val);
            }
            if (sets.length === 0) return localApi.objectives.get(id);
            sets.push('updated_at = ?');
            vals.push(now());
            vals.push(id);
            await execute(`UPDATE weekly_objectives SET ${sets.join(', ')} WHERE id = ?`, vals);
            await logMutation('objectives', id, 'update', patch as unknown as Record<string, unknown>);
            flushSoon();
            return localApi.objectives.get(id);
        },
        delete: async (id: string): Promise<void> => {
            await execute(`DELETE FROM weekly_objectives WHERE id = ?`, [id]);
            await logMutation('objectives', id, 'delete', {});
            flushSoon();
        },
    },

    plans: {
        get: async (date: string): Promise<DailyPlan> => {
            const rows = await query<DailyPlan[]>(`SELECT * FROM daily_plans WHERE plan_date = ?`, [date]);
            if (rows.length === 0) {
                return {
                    id: '', plan_date: date, status: 'pending',
                    intention: null, reflection: null, wins: null, shutdown_at: null,
                    created_at: now(), updated_at: now(),
                };
            }
            return rows[0];
        },
        upsert: async (date: string, input: UpsertPlanInput): Promise<DailyPlan> => {
            const existing = await query<DailyPlan[]>(`SELECT * FROM daily_plans WHERE plan_date = ?`, [date]);
            const ts = now();
            if (existing.length === 0) {
                const id = uuid();
                await execute(
                    `INSERT INTO daily_plans (id, plan_date, status, intention, reflection, wins, shutdown_at, created_at, updated_at)
                     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
                    [id, date, input.status ?? 'pending', input.intention ?? null,
                     input.reflection ?? null, input.wins ?? null, input.shutdown_at ?? null, ts, ts],
                );
            } else {
                const sets: string[] = [];
                const vals: unknown[] = [];
                for (const [key, val] of Object.entries(input).filter(([, v]) => v !== undefined)) {
                    sets.push(`${key} = ?`);
                    vals.push(val);
                }
                sets.push('updated_at = ?');
                vals.push(ts);
                vals.push(date);
                await execute(`UPDATE daily_plans SET ${sets.join(', ')} WHERE plan_date = ?`, vals);
            }
            const plan = await localApi.plans.get(date);
            // Plans upsert by date server-side (PUT /plans/{date}); entity_id is
            // the date and the payload carries plan_date for the replay.
            await logMutation('plans', date, 'update',
                { ...input, plan_date: date } as unknown as Record<string, unknown>);
            flushSoon();
            return plan;
        },
    },

    pomodoros: {
        create: async (input: {
            task_id: string; duration_minutes: number; started_at: string;
            completed_at?: string; was_completed: boolean;
        }): Promise<PomodoroSession> => {
            const id = uuid();
            const ts = now();
            await execute(
                `INSERT INTO pomodoro_sessions (id, task_id, duration_minutes, started_at, completed_at, was_completed, created_at)
                 VALUES (?, ?, ?, ?, ?, ?, ?)`,
                [id, input.task_id, input.duration_minutes, input.started_at,
                 input.completed_at ?? null, input.was_completed ? 1 : 0, ts],
            );
            const rows = await query<PomodoroSession[]>(`SELECT * FROM pomodoro_sessions WHERE id = ?`, [id]);
            return rows[0];
        },
        listByTask: async (taskId: string): Promise<PomodoroSession[]> => {
            return query<PomodoroSession[]>(
                `SELECT * FROM pomodoro_sessions WHERE task_id = ? ORDER BY started_at DESC`, [taskId],
            );
        },
    },

    tags: {
        list: async (): Promise<TagDefinition[]> => {
            return query<TagDefinition[]>(`SELECT * FROM tag_definitions ORDER BY name ASC`);
        },
        create: async (name: string, color?: string): Promise<TagDefinition> => {
            const id = uuid();
            const ts = now();
            await execute(
                `INSERT INTO tag_definitions (id, name, color, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
                [id, name, color ?? '#6366f1', ts, ts],
            );
            const rows = await query<TagDefinition[]>(`SELECT * FROM tag_definitions WHERE id = ?`, [id]);
            // Carry name/color in the payload; the replay sends id too so the
            // server row shares this id (avoids a duplicate-name conflict on pull).
            await logMutation('tags', id, 'create', { name, color: rows[0].color });
            flushSoon();
            return rows[0];
        },
        update: async (id: string, color: string): Promise<TagDefinition> => {
            await execute(`UPDATE tag_definitions SET color = ?, updated_at = ? WHERE id = ?`, [color, now(), id]);
            const rows = await query<TagDefinition[]>(`SELECT * FROM tag_definitions WHERE id = ?`, [id]);
            await logMutation('tags', id, 'update', { color });
            flushSoon();
            return rows[0];
        },
        delete: async (id: string): Promise<void> => {
            await execute(`DELETE FROM tag_definitions WHERE id = ?`, [id]);
            await logMutation('tags', id, 'delete', {});
            flushSoon();
        },
    },

    recurring: {
        list: async (): Promise<Task[]> => {
            const rows = await query<Record<string, unknown>[]>(
                `SELECT * FROM tasks WHERE recurrence_rule IS NOT NULL ORDER BY title ASC`,
            );
            return rows.map(parseTaskRow);
        },
        delete: async (id: string): Promise<void> => {
            await execute(`DELETE FROM tasks WHERE id = ?`, [id]);
            await logMutation('tasks', id, 'delete', {});
            flushSoon();
        },
    },

    weeks: {
        getReview: async (ws: string): Promise<WeekReview> => {
            const rows = await query<WeekReview[]>(`SELECT * FROM week_reviews WHERE week_start = ?`, [ws]);
            if (rows.length === 0) {
                return { week_start: ws, wins: null, challenges: null, next_focus: null };
            }
            return rows[0];
        },
        upsertReview: async (ws: string, data: { wins: string | null; challenges: string | null; next_focus: string | null }): Promise<WeekReview> => {
            const existing = await query<WeekReview[]>(`SELECT * FROM week_reviews WHERE week_start = ?`, [ws]);
            const ts = now();
            if (existing.length === 0) {
                const id = uuid();
                await execute(
                    `INSERT INTO week_reviews (id, week_start, wins, challenges, next_focus, created_at, updated_at)
                     VALUES (?, ?, ?, ?, ?, ?, ?)`,
                    [id, ws, data.wins, data.challenges, data.next_focus, ts, ts],
                );
            } else {
                await execute(
                    `UPDATE week_reviews SET wins = ?, challenges = ?, next_focus = ?, updated_at = ? WHERE week_start = ?`,
                    [data.wins, data.challenges, data.next_focus, ts, ws],
                );
            }
            // Week reviews upsert by week_start server-side (PUT /weeks/{ws}/review);
            // entity_id is the week_start and the payload carries it for the replay.
            await logMutation('week_reviews', ws, 'update',
                { ...data, week_start: ws } as unknown as Record<string, unknown>);
            flushSoon();
            return localApi.weeks.getReview(ws);
        },
    },

    ical: {
        listSubscriptions: async () => [],
        createSubscription: async () => { throw new Error('Not available in desktop mode'); },
        deleteSubscription: async () => {},
        syncSubscription: async () => ({ status: 'ok' }),
        listEvents: async () => [],
    },

    integrations: {
        jira: {
            get: async () => ({ connected: false }),
            save: async () => ({ connected: false }),
            test: async () => ({ status: 'ok' }),
            sync: async () => ({ total: 0, new: 0, updated: 0, errors: 0 }),
            delete: async () => {},
            getStatuses: async () => [],
            getIssue: async () => ({}),
            getTransitions: async () => [],
            transition: async () => {},
        },
        gmail: {
            get: async () => ({ connected: false }),
            authUrl: () => '#',
            updateLabels: async () => ({}),
            sync: async () => ({ total: 0, new: 0, updated: 0, errors: 0 }),
            delete: async () => {},
        },
        calendar: {
            get: async () => ({ connected: false }),
            toggle: async () => ({ enabled: false }),
            sync: async () => ({ total: 0, new: 0, updated: 0, errors: 0 }),
        },
        fastmail: {
            get: async () => ({ connected: false }),
            save: async () => ({}),
            sync: async () => ({ total: 0, new: 0, updated: 0, errors: 0 }),
            delete: async () => {},
            emails: async () => [],
            archivedEmails: async () => [],
            toTask: async () => { throw new Error('Not available in desktop mode'); },
            archive: async () => {},
            unarchive: async () => {},
            calendar: {
                get: async () => ({ connected: false, enabled: false }),
                toggle: async () => ({ enabled: false }),
                sync: async () => ({ synced: 0, from: '', to: '' }),
            },
        },
        caldav: {
            get: async () => ({ connected: false }),
            calendars: async () => [] as { href: string; name: string; color?: string }[],
            select: async () => ({ enabled: false, calendar_href: '', calendar_name: '' }),
            toggle: async () => ({ enabled: false }),
            sync: async () => ({ synced: 0 }),
        },
        taskInbox: {
            get: async () => ({ connected: false }),
            save: async () => ({ connected: false, email: '', inbox_address: '', allowed_senders: [] as string[] }),
            setSenders: async () => ({ allowed_senders: [] as string[] }),
            sync: async () => ({ total: 0, new: 0, updated: 0, errors: 0 }),
            delete: async () => {},
        },
        emailForward: {
            get: async () => ({ smtp_enabled: false, smtp_address: '', smtp_port: '', webhook_enabled: false, webhook_url: '' }),
        },
    },
};
