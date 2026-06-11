/**
 * On-device scheduled reminders for Android (Capacitor).
 *
 * This is the reliability backbone: it schedules a real OS alarm
 * (@capacitor/local-notifications) for every upcoming task reminder, read
 * straight from the LOCAL database. Because the OS holds the alarm, the
 * reminder fires even when:
 *   • the server is unreachable,
 *   • the device is fully offline, or
 *   • the app is closed.
 *
 * The server-side Web Push / FCM remains as a redundant channel. Per the
 * "prefer no-miss" choice we tolerate a rare duplicate rather than risk a miss,
 * so no cross-channel suppression is attempted.
 *
 * Scheduling is idempotent and diff-based: we keep a map of {taskId → alarm} in
 * localStorage and only (re)schedule what changed, cancelling alarms for tasks
 * that were completed, deleted, or had their reminder cleared/moved.
 */

import { isCapacitor } from '$lib/platform';
import { localApi } from '$lib/tauri/local-api';
import { notificationSettings } from '$lib/stores/notificationSettings.svelte';
import { DEFAULT_SOUND_ID, NOTIFICATION_SOUNDS } from '$lib/sounds';

const MAP_KEY = 'sempa-local-reminder-map';

interface ScheduledEntry {
  notifId: number;
  remindAt: string;
  title: string;
  soundId: string;
}
type ScheduleMap = Record<string, ScheduledEntry>;

let navigate: (url: string) => void = () => {};
let listenersBound = false;
let running = false;
let rerunQueued = false;

// Stable positive 31-bit int from a task UUID (local-notifications needs ints).
function notifIdFor(uuid: string): number {
  let h = 5381;
  for (let i = 0; i < uuid.length; i++) h = ((h << 5) + h + uuid.charCodeAt(i)) | 0;
  return Math.abs(h) % 2147483646 + 1;
}

function readMap(): ScheduleMap {
  try {
    return JSON.parse(localStorage.getItem(MAP_KEY) || '{}');
  } catch {
    return {};
  }
}
function writeMap(m: ScheduleMap) {
  try {
    localStorage.setItem(MAP_KEY, JSON.stringify(m));
  } catch {
    /* ignore */
  }
}

type LocalNotifModule = typeof import('@capacitor/local-notifications');
type LocalNotif = LocalNotifModule['LocalNotifications'];

async function loadPlugin(): Promise<LocalNotif | null> {
  try {
    const mod = await import('@capacitor/local-notifications');
    return mod.LocalNotifications;
  } catch {
    return null;
  }
}

async function ensurePermission(LN: LocalNotif): Promise<boolean> {
  try {
    let perm = await LN.checkPermissions();
    if (perm.display !== 'granted') perm = await LN.requestPermissions();
    return perm.display === 'granted';
  } catch {
    return false;
  }
}

// Make sure the per-sound channel exists so the chosen tone plays.
async function ensureChannel(LN: LocalNotif, soundId: string) {
  if (!LN.createChannel) return;
  const label = NOTIFICATION_SOUNDS.find((s) => s.id === soundId)?.label ?? soundId;
  try {
    await LN.createChannel({
      id: `snd_${soundId}`,
      name: `Reminder — ${label}`,
      description: 'Sempa task reminders',
      sound: soundId, // res/raw resource name (no extension) — matches push.ts
      importance: 5,
      visibility: 1,
      vibration: true,
    });
  } catch {
    /* already exists / unsupported */
  }
}

async function bindListeners(LN: LocalNotif) {
  if (listenersBound) return;
  listenersBound = true;
  try {
    await LN.registerActionTypes({
      types: [
        {
          id: 'REMINDER',
          actions: [
            { id: 'done', title: 'Mark done' },
            { id: 'snooze', title: 'Snooze 1h' },
          ],
        },
      ],
    });
    await LN.addListener('localNotificationActionPerformed', async (event) => {
      const extra = (event.notification.extra ?? {}) as { taskId?: string; url?: string };
      const taskId = extra.taskId;
      if (!taskId) return;
      // api.tasks on Capacitor writes the local DB and queues a sync, so these
      // work offline and reconcile with the server on reconnect.
      const { api } = await import('$lib/api');
      if (event.actionId === 'done') {
        await api.tasks.update(taskId, { status: 'done' }).catch(() => {});
        void syncLocalReminders();
      } else if (event.actionId === 'snooze') {
        const at = new Date(Date.now() + 60 * 60 * 1000).toISOString();
        await api.tasks.update(taskId, { remind_at: at }).catch(() => {});
        void syncLocalReminders();
      } else {
        // Body tap → deep-link into the app.
        navigate(extra.url || `/focus/${taskId}`);
      }
    });
  } catch {
    /* plugin without action support — basic notifications still work */
  }
}

/**
 * Reconcile scheduled OS alarms with the current local DB + settings.
 * Safe to call often; coalesces concurrent invocations.
 */
export async function syncLocalReminders(): Promise<void> {
  if (!isCapacitor()) return;
  if (running) {
    rerunQueued = true;
    return;
  }
  running = true;
  try {
    const LN = await loadPlugin();
    if (!LN) return;

    const st = notificationSettings.settings;
    const remindersOn = st.master_enabled; // master gate
    const soundOn = st.master_enabled && st.sound_enabled;
    const soundId = st.sound_id || DEFAULT_SOUND_ID;

    if (!(await ensurePermission(LN))) return;
    await bindListeners(LN);
    if (soundOn) await ensureChannel(LN, soundId);

    const prev = readMap();
    const next: ScheduleMap = {};
    const toSchedule: Parameters<LocalNotif['schedule']>[0]['notifications'] = [];
    const toCancel: { id: number }[] = [];

    if (remindersOn) {
      const tasks = await localApi.tasks.withReminders();
      const now = Date.now();
      for (const t of tasks) {
        if (!t.remind_at) continue;
        const when = new Date(t.remind_at).getTime();
        if (isNaN(when) || when <= now) continue; // past-due handled by server catch-up
        const notifId = notifIdFor(t.id);
        const entry: ScheduledEntry = { notifId, remindAt: t.remind_at, title: t.title, soundId: soundOn ? soundId : '' };
        next[t.id] = entry;

        const unchanged =
          prev[t.id] &&
          prev[t.id].remindAt === entry.remindAt &&
          prev[t.id].title === entry.title &&
          prev[t.id].soundId === entry.soundId;
        if (unchanged) continue; // already scheduled correctly

        toSchedule.push({
          id: notifId,
          title: 'Reminder',
          body: t.title,
          schedule: { at: new Date(when), allowWhileIdle: true },
          channelId: soundOn ? `snd_${soundId}` : undefined,
          actionTypeId: 'REMINDER',
          extra: { taskId: t.id, url: `/focus/${t.id}` },
        });
      }
    }

    // Cancel alarms for tasks that disappeared or whose reminder changed (the
    // changed ones are re-added above with the same id, which replaces them).
    for (const taskId of Object.keys(prev)) {
      if (!next[taskId]) toCancel.push({ id: prev[taskId].notifId });
    }

    if (toCancel.length) await LN.cancel({ notifications: toCancel }).catch(() => {});
    if (toSchedule.length) await LN.schedule({ notifications: toSchedule }).catch(() => {});
    writeMap(next);
  } catch {
    /* best-effort; the server push channel is the backup */
  } finally {
    running = false;
    if (rerunQueued) {
      rerunQueued = false;
      void syncLocalReminders();
    }
  }
}

/** Wire deep-link navigation and run an initial schedule. Capacitor only. */
export function initLocalReminders(nav: (url: string) => void): void {
  if (!isCapacitor()) return;
  navigate = nav;
  void syncLocalReminders();
}
