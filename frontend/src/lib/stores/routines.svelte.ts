/**
 * In-app scheduled routines — Weekly Planning prompt + Daily Shutdown review.
 *
 * These are deliberately NOT OS notifications: they surface as a dismissible
 * in-app banner (RoutineBanner.svelte) so they feel like a focus space, not an
 * alarm. The Monday-morning planning prompt and end-of-day shutdown prompt fire
 * at times the user configures in Notification settings.
 *
 * State management — no leaks, no busy polling:
 *   • A SINGLE setTimeout is armed to the next trigger boundary and re-armed
 *     after each fire. There is no setInterval.
 *   • The timer is cleared on destroy() and recomputed when settings change.
 *   • We also re-evaluate on focus / visibilitychange so a machine that slept
 *     through a trigger catches up the moment it wakes.
 *
 * On Tauri desktop (which can't receive Web Push), the same timer doubles as a
 * lightweight reminder check: due task reminders fire a native OS notification
 * via @tauri-apps/plugin-notification while the app is running.
 */

import { api } from '$lib/api';
import { today, weekStart } from '$lib/utils';
import { isTauri } from '$lib/platform';
import { localApi } from '$lib/tauri/local-api';
import { playSound, DEFAULT_SOUND_ID } from '$lib/sounds';
import type { NotificationSettings } from '$lib/types';

const DEFAULTS: NotificationSettings['routines'] = {
  weekly_plan_day: 1,
  weekly_plan_time: '08:30',
  daily_shutdown_time: '17:00',
  workdays: [1, 2, 3, 4, 5],
};

// localStorage keys remembering that a prompt was dismissed for a given period.
const planDismissKey = (ws: string) => `sempa-routine-plan-${ws}`;
const shutdownDismissKey = (d: string) => `sempa-routine-shutdown-${d}`;
const NOTIFIED_KEY = 'sempa-tauri-notified-reminders';

const SIX_HOURS = 6 * 60 * 60 * 1000;
const ONE_MINUTE = 60 * 1000;

function createRoutinesStore() {
  let routines = $state<NotificationSettings['routines']>(DEFAULTS);
  let masterEnabled = $state(true);
  let soundEnabled = $state(true);
  let soundId = $state(DEFAULT_SOUND_ID);

  let weeklyPlanDue = $state(false);
  let shutdownDue = $state(false);

  let navigate: (url: string) => void = () => {};
  let timer: ReturnType<typeof setTimeout> | null = null;
  let started = false;
  let onVisibility: (() => void) | null = null;

  // ── ISO day-of-week: 1=Mon … 7=Sun ────────────────────────────────────────
  function isoDow(d: Date): number {
    const js = d.getDay(); // 0=Sun … 6=Sat
    return js === 0 ? 7 : js;
  }

  function parseHM(hm: string): [number, number] {
    const [h, m] = (hm || '00:00').split(':').map((n) => parseInt(n, 10));
    return [isNaN(h) ? 0 : h, isNaN(m) ? 0 : m];
  }

  function atTime(base: Date, hm: string): Date {
    const [h, m] = parseHM(hm);
    const d = new Date(base);
    d.setHours(h, m, 0, 0);
    return d;
  }

  // ── Evaluate whether either prompt should currently be showing ─────────────
  function evaluate() {
    if (!masterEnabled) {
      weeklyPlanDue = false;
      shutdownDue = false;
      return;
    }
    const now = new Date();
    const dow = isoDow(now);
    const wasDue = weeklyPlanDue || shutdownDue;

    // Weekly planning: on the configured weekday, any time after the set time,
    // until dismissed for this week.
    const planTime = atTime(now, routines.weekly_plan_time);
    const ws = weekStart(today());
    weeklyPlanDue =
      dow === routines.weekly_plan_day &&
      now >= planTime &&
      localStorage.getItem(planDismissKey(ws)) !== '1';

    // Daily shutdown: on a workday, after the shutdown time, until dismissed today.
    const shutdownTime = atTime(now, routines.daily_shutdown_time);
    const td = today();
    shutdownDue =
      routines.workdays.includes(dow) &&
      now >= shutdownTime &&
      localStorage.getItem(shutdownDismissKey(td)) !== '1';

    // Rising edge: a banner just appeared → gentle audible cue (foreground only).
    if (!wasDue && (weeklyPlanDue || shutdownDue) && soundEnabled) {
      playSound(soundId);
    }

    if (isTauri()) void checkTauriReminders();
  }

  // ── Compute the soonest upcoming trigger so we can arm an exact timeout ─────
  function msUntilNextTrigger(): number {
    const now = new Date();
    const candidates: number[] = [];

    // Next occurrence of the weekly planning time.
    for (let i = 0; i <= 7; i++) {
      const d = new Date(now);
      d.setDate(now.getDate() + i);
      if (isoDow(d) === routines.weekly_plan_day) {
        const t = atTime(d, routines.weekly_plan_time);
        if (t > now) candidates.push(t.getTime() - now.getTime());
      }
    }
    // Next workday shutdown time.
    for (let i = 0; i <= 7; i++) {
      const d = new Date(now);
      d.setDate(now.getDate() + i);
      if (routines.workdays.includes(isoDow(d))) {
        const t = atTime(d, routines.daily_shutdown_time);
        if (t > now) candidates.push(t.getTime() - now.getTime());
      }
    }

    const soonest = candidates.length ? Math.min(...candidates) : SIX_HOURS;
    // On Tauri the timer also drives the reminder poll, so check at least once a
    // minute; otherwise cap at 6h so an idle app re-evaluates periodically.
    const cap = isTauri() ? ONE_MINUTE : SIX_HOURS;
    return Math.max(1000, Math.min(soonest, cap));
  }

  function arm() {
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => {
      evaluate();
      arm();
    }, msUntilNextTrigger());
  }

  // ── Tauri desktop: fire native OS notifications for due task reminders ──────
  async function checkTauriReminders() {
    try {
      const due = await localApi.tasks.dueReminders();
      if (!due.length) return;
      const notified = new Set<string>(
        JSON.parse(localStorage.getItem(NOTIFIED_KEY) || '[]'),
      );
      const mod = await import('@tauri-apps/plugin-notification');
      let granted = await mod.isPermissionGranted();
      if (!granted) granted = (await mod.requestPermission()) === 'granted';
      if (!granted) return;

      for (const t of due) {
        if (notified.has(t.id)) continue;
        // Windows toasts can't carry a custom audio file, so when a sound is
        // chosen we silence the toast and play the selected tone via the
        // WebView's audio instead (the app is running, so this works).
        mod.sendNotification({ title: 'Reminder', body: t.title, silent: soundEnabled });
        if (soundEnabled) playSound(soundId);
        notified.add(t.id);
      }
      // Keep the notified set bounded.
      localStorage.setItem(NOTIFIED_KEY, JSON.stringify([...notified].slice(-200)));
    } catch {
      // Plugin unavailable or offline — silently skip; web push covers browsers.
    }
  }

  async function loadSettings() {
    try {
      const s = await api.notifications.getSettings();
      routines = s.routines ?? DEFAULTS;
      masterEnabled = s.master_enabled;
      soundEnabled = s.sound_enabled;
      soundId = s.sound_id || DEFAULT_SOUND_ID;
    } catch {
      // Offline / no server — fall back to defaults so prompts still work.
      routines = DEFAULTS;
      masterEnabled = true;
      soundEnabled = true;
      soundId = DEFAULT_SOUND_ID;
    }
  }

  // ── Public API ─────────────────────────────────────────────────────────────
  async function init(nav: (url: string) => void) {
    navigate = nav;
    if (started) {
      evaluate();
      return;
    }
    started = true;

    onVisibility = () => {
      if (typeof document !== 'undefined' && !document.hidden) evaluate();
    };
    document.addEventListener('visibilitychange', onVisibility);
    window.addEventListener('focus', onVisibility);

    await loadSettings();
    evaluate();
    arm();
  }

  /** Re-pull settings (e.g. after the user edits the routine schedule). */
  async function refresh() {
    await loadSettings();
    evaluate();
    arm();
  }

  function startWeeklyPlan() {
    const ws = weekStart(today());
    localStorage.setItem(planDismissKey(ws), '1');
    weeklyPlanDue = false;
    navigate(`/week/${ws}/plan`);
  }

  function dismissWeeklyPlan() {
    localStorage.setItem(planDismissKey(weekStart(today())), '1');
    weeklyPlanDue = false;
  }

  function startShutdown() {
    const td = today();
    localStorage.setItem(shutdownDismissKey(td), '1');
    shutdownDue = false;
    navigate(`/shutdown/${td}`);
  }

  function dismissShutdown() {
    localStorage.setItem(shutdownDismissKey(today()), '1');
    shutdownDue = false;
  }

  function destroy() {
    if (timer) clearTimeout(timer);
    timer = null;
    if (onVisibility) {
      document.removeEventListener('visibilitychange', onVisibility);
      window.removeEventListener('focus', onVisibility);
      onVisibility = null;
    }
    started = false;
  }

  return {
    get weeklyPlanDue() { return weeklyPlanDue; },
    get shutdownDue() { return shutdownDue; },
    init,
    refresh,
    startWeeklyPlan,
    dismissWeeklyPlan,
    startShutdown,
    dismissShutdown,
    destroy,
  };
}

export const routines = createRoutinesStore();
