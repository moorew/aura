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
 * lightweight reminder check: due task reminders are pushed to the shared
 * reminderAlerts store, which surfaces a Granola-style floating card in a
 * separate always-on-top window (see $lib/desktopReminderPopup), plus the
 * chosen tone, while the app is running.
 */

import { today, weekStart } from '$lib/utils';
import { isTauri, hasLocalDb } from '$lib/platform';
import { localApi } from '$lib/tauri/local-api';
import { playSound, DEFAULT_SOUND_ID } from '$lib/sounds';
import { notificationSettings } from '$lib/stores/notificationSettings.svelte';
import { reminderAlerts } from '$lib/stores/reminderAlerts.svelte';
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

  // Sound + master are read LIVE from notificationSettings at fire time (see
  // liveSound()) rather than snapshotted: on desktop the settings store returns
  // its cached copy instantly and reconciles with the server in the background,
  // so a snapshot taken at startup would keep playing the stale default tone.
  function liveSound() {
    const s = notificationSettings.settings;
    return {
      master: s.master_enabled,
      soundOn: s.master_enabled && s.sound_enabled,
      soundId: s.sound_id || DEFAULT_SOUND_ID,
    };
  }

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
    const { master, soundOn, soundId } = liveSound();
    if (!master) {
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
    if (!wasDue && (weeklyPlanDue || shutdownDue) && soundOn) {
      playSound(soundId);
    }

    // Surface due task reminders in-app (and natively on desktop). Runs on any
    // local-first client (Tauri desktop + Android), which both have the local DB.
    if (hasLocalDb()) void checkDueReminders();
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
    // On local-first clients the timer also drives the due-reminder poll, so
    // check at least once a minute; otherwise cap at 6h so an idle app
    // re-evaluates periodically.
    const cap = hasLocalDb() ? ONE_MINUTE : SIX_HOURS;
    return Math.max(1000, Math.min(soonest, cap));
  }

  function arm() {
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => {
      evaluate();
      arm();
    }, msUntilNextTrigger());
  }

  // ── Local-first: surface due task reminders in-app + natively (desktop) ─────
  async function checkDueReminders() {
    try {
      const { master, soundOn, soundId } = liveSound();
      if (!master) return;

      const due = await localApi.tasks.dueReminders();
      if (!due.length) return;

      // Dedup keyed by task + its remind_at, so editing/snoozing a reminder
      // (which moves remind_at) legitimately re-fires, but the same firing
      // doesn't repeat every minute.
      const notified = new Set<string>(
        JSON.parse(localStorage.getItem(NOTIFIED_KEY) || '[]'),
      );
      const fresh = due.filter((t) => !notified.has(`${t.id}|${t.remind_at}`));
      if (!fresh.length) return;

      // Push to the shared alert list. On desktop this drives the floating
      // top-right card (see $lib/desktopReminderPopup, wired from +layout); on
      // web it drives the in-app banner; on Android the on-device OS alarm
      // already fired, and the banner backs it up.
      for (const t of fresh) {
        reminderAlerts.push(t);
        notified.add(`${t.id}|${t.remind_at}`);
      }

      // Desktop: play the chosen tone as the audible cue for the floating card
      // (the WebView is running, so custom audio works where a Windows toast
      // couldn't). Android already played its channel sound with the OS alarm.
      if (isTauri() && soundOn) playSound(soundId);

      // Keep the notified set bounded.
      localStorage.setItem(NOTIFIED_KEY, JSON.stringify([...notified].slice(-200)));
    } catch {
      // Plugin unavailable or offline — silently skip; web push covers browsers.
    }
  }

  async function loadSettings() {
    // Read from the local-first settings store so routines work offline too.
    // Only the routine schedule is snapshotted here (it drives the timer); sound
    // + master are read live at fire time via liveSound().
    await notificationSettings.init();
    routines = notificationSettings.settings.routines ?? DEFAULTS;
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
