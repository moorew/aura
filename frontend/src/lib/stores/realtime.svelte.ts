import { isTauri } from '$lib/tauri/bridge';
import { getTauriToken, getServerUrl } from '$lib/api';

export type ChangeEvent = {
  type: string;
  date?: string;
  week_start?: string;
  entity?: string;
};

function buildSSEUrl(): string {
  const base = isTauri() ? getServerUrl() : '';
  const url = new URL(`${base}/api/v1/events`, window.location.href);
  if (isTauri()) {
    const token = getTauriToken();
    if (token) url.searchParams.set('token', token);
  }
  return url.toString();
}

function createRealtimeStore() {
  let es: EventSource | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  let reconnectDelay = 1000;
  let connected = $state(false);
  let lastEvent = $state<ChangeEvent | null>(null);
  let listeners: Set<(ev: ChangeEvent) => void> = new Set();
  let reconnectListeners: Set<() => void> = new Set();

  function connect() {
    if (es) return;
    try {
      es = new EventSource(buildSSEUrl(), { withCredentials: !isTauri() });

      es.addEventListener('change', (e: MessageEvent) => {
        try {
          const ev: ChangeEvent = JSON.parse(e.data);
          lastEvent = ev;
          reconnectDelay = 1000; // reset backoff on success
          listeners.forEach(fn => fn(ev));
        } catch { /* ignore parse errors */ }
      });

      es.onopen = () => {
        const wasReconnect = reconnectDelay > 1000; // we'd backed off → this is a recovery
        connected = true;
        reconnectDelay = 1000; // reset backoff
        // A successful SSE (re)open proves the server is reachable again — the
        // most reliable "back online" signal on Tailscale, where the OS 'online'
        // event doesn't fire for tailnet reconnects. Nudge a sync so queued
        // local changes push and remote changes pull within ~a second, with no
        // manual Sync press. Fires on first connect too; sync() is coalesced so
        // the duplicate is harmless.
        if (wasReconnect) reconnectListeners.forEach(fn => fn());
      };

      es.onerror = () => {
        connected = false;
        es?.close();
        es = null;
        // Exponential backoff, cap at 30s
        reconnectDelay = Math.min(reconnectDelay * 2, 30_000);
        if (reconnectTimer) clearTimeout(reconnectTimer);
        reconnectTimer = setTimeout(connect, reconnectDelay);
      };
    } catch {
      // EventSource not available (e.g., server URL not configured yet)
    }
  }

  function disconnect() {
    if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null; }
    es?.close();
    es = null;
    connected = false;
  }

  function subscribe(fn: (ev: ChangeEvent) => void): () => void {
    listeners.add(fn);
    return () => listeners.delete(fn);
  }

  // Register a callback fired when the SSE connection RECOVERS after a drop
  // (i.e. the server became reachable again). Used to auto-trigger a sync.
  function onReconnect(fn: () => void): () => void {
    reconnectListeners.add(fn);
    return () => reconnectListeners.delete(fn);
  }

  // Inject a synthetic change event. Used by the local-first sync engine after a
  // pull writes rows: pages already re-read on `task:change`/`objective:change`,
  // so routing local-DB updates through the same channel makes them refresh
  // without every page having to also watch the sync store directly.
  function emitLocal(type: string) {
    const ev: ChangeEvent = { type };
    lastEvent = ev;
    listeners.forEach(fn => fn(ev));
  }

  return {
    get connected() { return connected; },
    get lastEvent() { return lastEvent; },
    connect,
    disconnect,
    subscribe,
    onReconnect,
    emitLocal,
  };
}

export const realtime = createRealtimeStore();
