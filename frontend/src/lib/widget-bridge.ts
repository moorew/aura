/**
 * Bridge to push task data to Android Glance widgets via the WidgetBridge Capacitor plugin.
 * Falls back silently on web/iOS where the plugin is not available.
 */

import type { Task } from './types';
import { today, offsetDate, weekStart } from './utils';

interface WidgetBridgePlugin {
  updateWidgetData(opts: {
    todayTotal: number;
    todayDone: number;
    tasks: { title: string; done: boolean }[];
    week: { date: string; count: number }[];
  }): Promise<void>;
}

function getPlugin(): WidgetBridgePlugin | null {
  try {
    // Capacitor registers plugins on window.Capacitor.Plugins
    const cap = (window as any).Capacitor;
    if (cap?.Plugins?.WidgetBridge) {
      return cap.Plugins.WidgetBridge as WidgetBridgePlugin;
    }
  } catch {}
  return null;
}

/**
 * Sync current task data to the Android widget SharedPreferences.
 * Call this after task list changes (create, complete, delete, reorder).
 */
export function syncWidgetData(todayTasks: Task[], weekTaskCounts?: Map<string, number>) {
  const plugin = getPlugin();
  if (!plugin) return;

  const todayDate = today();
  const total = todayTasks.length;
  const done = todayTasks.filter(t => t.status === 'done').length;
  const tasks = todayTasks.slice(0, 10).map(t => ({
    title: t.title,
    done: t.status === 'done',
  }));

  // Build week data
  const ws = weekStart(todayDate);
  const week: { date: string; count: number }[] = [];
  for (let i = 0; i < 7; i++) {
    const d = offsetDate(ws, i);
    week.push({ date: d, count: weekTaskCounts?.get(d) ?? 0 });
  }

  plugin.updateWidgetData({ todayTotal: total, todayDone: done, tasks, week }).catch(() => {});
}
