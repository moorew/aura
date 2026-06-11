/**
 * Desktop (Tauri) reminder popup controller — runs in the MAIN window only.
 *
 * The Granola-style floating card is a separate, always-on-top Tauri window
 * (src-tauri/src/windows.rs `create_reminder_popup`, route `/reminder-popup`).
 * This module keeps that window in sync with the shared `reminderAlerts` store
 * and routes the card's actions back into the app:
 *   • alerts present  → show the window + emit the current list to it
 *   • alerts empty    → close the window
 *   • card "open"     → surface the main window + deep-link to the task
 *   • done / dismiss / snooze → mutate `reminderAlerts` (which re-emits)
 *
 * The popup mirrors the in-app banner's data, so on desktop we show ONLY the
 * floating card (the in-app banner is suppressed) — one notification, visible
 * even when Sempa is in the background.
 */

import { isTauri } from '$lib/platform';
import { reminderAlerts, type ReminderAlert } from '$lib/stores/reminderAlerts.svelte';

let actionsBound = false;
let navigateFn: (url: string) => void = () => {};

function toCards(alerts: ReminderAlert[]) {
  return alerts.map((a) => ({
    taskId: a.taskId,
    title: a.title,
    subtitle:
      'Reminder · ' +
      new Date(a.at).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' }),
  }));
}

/** Reconcile the popup window with the current alert list. Call on every change. */
export async function syncDesktopPopup(alerts: ReminderAlert[]): Promise<void> {
  if (!isTauri()) return;
  try {
    const { invoke } = await import('@tauri-apps/api/core');
    if (alerts.length === 0) {
      await invoke('close_reminder_popup').catch(() => {});
      return;
    }
    await invoke('show_reminder_popup').catch(() => {});
    const { emit } = await import('@tauri-apps/api/event');
    await emit('reminder:list', toCards(alerts));
  } catch {
    /* Tauri API unavailable — in-app banner / native path still covers it */
  }
}

/** Bind the popup→main action + ready listeners once. Main window only. */
export async function initDesktopReminderPopup(navigate: (url: string) => void): Promise<void> {
  if (!isTauri() || actionsBound) return;
  actionsBound = true;
  navigateFn = navigate;

  try {
    const { listen, emit } = await import('@tauri-apps/api/event');

    // The popup announces itself on mount → (re)send the current list so the
    // first card never gets lost to a create/emit race.
    await listen('reminder:ready', async () => {
      await emit('reminder:list', toCards(reminderAlerts.alerts));
    });

    await listen<{ action: string; taskId: string }>('reminder:action', async (e) => {
      const { action, taskId } = e.payload;
      if (action === 'open') {
        // Surface the main window (it's hidden-to-tray, not minimized) then
        // deep-link into the task.
        try {
          const { getCurrentWindow } = await import('@tauri-apps/api/window');
          const w = getCurrentWindow();
          await w.show().catch(() => {});
          await w.unminimize().catch(() => {});
          await w.setFocus().catch(() => {});
        } catch {
          /* ignore — navigation still happens */
        }
        navigateFn(`/focus/${taskId}`);
        reminderAlerts.dismiss(taskId);
      } else if (action === 'done') {
        void reminderAlerts.markDone(taskId);
      } else if (action === 'snooze') {
        void reminderAlerts.snooze(taskId);
      } else {
        reminderAlerts.dismiss(taskId);
      }
    });
  } catch {
    /* Tauri event API unavailable */
  }
}
