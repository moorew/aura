/**
 * Tauri IPC bridge — provides type-safe wrappers around Tauri commands.
 * Falls back gracefully when running in a browser (non-Tauri) context.
 */

interface TauriInvoke {
    (cmd: string, args?: Record<string, unknown>): Promise<unknown>;
}

interface TauriEvent {
    listen(event: string, handler: (event: { payload: unknown }) => void): Promise<() => void>;
}

function getTauri(): { invoke: TauriInvoke; event: TauriEvent } | null {
    if (typeof window !== 'undefined' && '__TAURI__' in window) {
        const t = (window as any).__TAURI__;
        return {
            invoke: t.core.invoke,
            event: t.event,
        };
    }
    return null;
}

export function isTauri(): boolean {
    return typeof window !== 'undefined' && '__TAURI__' in window;
}

// Platform helpers (canonical definitions live in $lib/platform). Re-exported
// here so callers that already import from the bridge get them in one place.
export { isCapacitor, hasLocalDb } from '$lib/platform';

// ── Commands ────────────────────────────────────────────────────────────────

export async function triggerSync(): Promise<void> {
    const t = getTauri();
    if (t) await t.invoke('trigger_sync');
}

export async function getSyncStatus(): Promise<SyncStatus> {
    const t = getTauri();
    if (t) return (await t.invoke('get_sync_status')) as SyncStatus;
    return { syncing: false, last_synced_at: null, pending_mutations: 0, online: false };
}

export async function getServerUrl(): Promise<string> {
    const t = getTauri();
    if (t) return (await t.invoke('get_server_url')) as string;
    return '';
}

export async function setServerUrl(url: string): Promise<void> {
    const t = getTauri();
    if (t) await t.invoke('set_server_url', { url });
}

export async function updateTaskbarBadge(count: number): Promise<void> {
    const t = getTauri();
    if (t) await t.invoke('update_taskbar_badge', { count });
}

export async function quickAddTask(title: string, plannedDate?: string): Promise<string | null> {
    const t = getTauri();
    if (t) {
        return (await t.invoke('quick_add_task', {
            task: { title, planned_date: plannedDate ?? null },
        })) as string;
    }
    return null;
}

export async function createWidgetWindow(): Promise<void> {
    const t = getTauri();
    if (t) await t.invoke('create_widget_window');
}

export async function createStickyNote(
    noteId: string,
    x = 100,
    y = 100,
    width = 240,
    height = 200,
): Promise<void> {
    const t = getTauri();
    if (t) await t.invoke('create_sticky_note', { noteId, x, y, width, height });
}

export async function closeStickyNote(noteId: string): Promise<void> {
    const t = getTauri();
    if (t) await t.invoke('close_sticky_note', { noteId });
}

export interface StickyPosition {
    note_id: string;
    x: number;
    y: number;
    width: number;
    height: number;
}

export async function saveStickyPositions(positions: StickyPosition[]): Promise<void> {
    const t = getTauri();
    if (t) await t.invoke('save_sticky_positions', { positions });
}

export async function getStickyPositions(): Promise<StickyPosition[]> {
    const t = getTauri();
    if (t) return (await t.invoke('get_sticky_positions')) as StickyPosition[];
    return [];
}

// ── Event listeners ─────────────────────────────────────────────────────────

export interface SyncStatus {
    syncing: boolean;
    last_synced_at: string | null;
    pending_mutations: number;
    online: boolean;
}

export async function onSyncStatus(
    handler: (status: SyncStatus) => void,
): Promise<(() => void) | null> {
    const t = getTauri();
    if (t) {
        return await t.event.listen('sync-status', (e) => handler(e.payload as SyncStatus));
    }
    return null;
}

/** Fires when the tray "Sync Now" item is clicked (Rust emits 'sync-trigger'). */
export async function onSyncTrigger(handler: () => void): Promise<(() => void) | null> {
    const t = getTauri();
    if (t) {
        return await t.event.listen('sync-trigger', () => handler());
    }
    return null;
}

export async function onTrayQuickAdd(handler: () => void): Promise<(() => void) | null> {
    const t = getTauri();
    if (t) {
        return await t.event.listen('tray-quick-add', () => handler());
    }
    return null;
}

export async function onQuickAddTask(
    handler: (task: { title: string; planned_date: string | null }) => void,
): Promise<(() => void) | null> {
    const t = getTauri();
    if (t) {
        return await t.event.listen('quick-add-task', (e) =>
            handler(e.payload as { title: string; planned_date: string | null }),
        );
    }
    return null;
}
