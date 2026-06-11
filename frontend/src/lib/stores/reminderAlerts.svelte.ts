/**
 * In-app fired-reminder alerts.
 *
 * When a task's hard reminder comes due while the app is open, we surface a
 * dismissible in-app banner (ReminderBanner.svelte) showing WHICH task rang —
 * so the user isn't left guessing after only hearing the tone. This is the
 * reliable, cross-platform visual: on Windows the native toast can be silently
 * suppressed (focus assist, missing toast registration), and the OS sound alone
 * gives no context. The banner always appears while the app is foregrounded.
 *
 * It is fed by the reminder poll in routines.svelte.ts (which reads due
 * reminders straight from the local DB on Tauri/Android). Actions route through
 * `api.tasks`, so Done/Snooze work offline and reconcile on reconnect.
 */

import type { Task } from '$lib/types';

export interface ReminderAlert {
  taskId: string;
  title: string;
  at: number; // when it surfaced (ms)
}

function createReminderAlertsStore() {
  let alerts = $state<ReminderAlert[]>([]);

  function push(task: Pick<Task, 'id' | 'title'>) {
    if (alerts.some((a) => a.taskId === task.id)) return;
    alerts = [...alerts, { taskId: task.id, title: task.title, at: Date.now() }];
  }

  function dismiss(taskId: string) {
    alerts = alerts.filter((a) => a.taskId !== taskId);
  }

  function clear() {
    alerts = [];
  }

  async function markDone(taskId: string) {
    dismiss(taskId);
    const { api } = await import('$lib/api');
    await api.tasks.update(taskId, { status: 'done' }).catch(() => {});
  }

  async function snooze(taskId: string, minutes = 60) {
    dismiss(taskId);
    const { api } = await import('$lib/api');
    const at = new Date(Date.now() + minutes * 60 * 1000).toISOString();
    await api.tasks.update(taskId, { remind_at: at }).catch(() => {});
  }

  return {
    get alerts() {
      return alerts;
    },
    push,
    dismiss,
    clear,
    markDone,
    snooze,
  };
}

export const reminderAlerts = createReminderAlertsStore();
