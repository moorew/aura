/**
 * Local-first notification settings.
 *
 * Settings are a single small global document. Rather than bolt them onto the
 * entity sync engine, we cache them in localStorage so the UI is instant and
 * works fully offline, and push changes to the server best-effort (last-write-
 * wins). A pending flag is flushed when connectivity returns.
 *
 * This is what lets the Notifications screen render immediately (no spinner /
 * hang) when the server is briefly unreachable, and keeps an offline edit from
 * being lost.
 */

import { api } from '$lib/api';
import type { NotificationSettings } from '$lib/types';
import { DEFAULT_SOUND_ID } from '$lib/sounds';

const CACHE_KEY = 'sempa-notification-settings';
const PENDING_KEY = 'sempa-notification-settings-pending';

export function defaultNotificationSettings(): NotificationSettings {
  return {
    master_enabled: true,
    webpush_enabled: true,
    fcm_enabled: true,
    webhook_enabled: false,
    sound_enabled: true,
    sound_id: DEFAULT_SOUND_ID,
    morning_digest: true,
    digest_hour: 8,
    webhook: { endpoint: '', method: 'POST', auth_header: '', auth_value: '', topic: '' },
    routines: {
      weekly_plan_day: 1,
      weekly_plan_time: '08:30',
      daily_shutdown_time: '17:00',
      workdays: [1, 2, 3, 4, 5],
    },
  };
}

function readCache(): NotificationSettings {
  try {
    const raw = localStorage.getItem(CACHE_KEY);
    if (raw) return { ...defaultNotificationSettings(), ...JSON.parse(raw) };
  } catch {
    /* ignore */
  }
  return defaultNotificationSettings();
}

function createNotificationSettingsStore() {
  let settings = $state<NotificationSettings>(defaultNotificationSettings());
  let loaded = $state(false);
  let started = false;

  function persist() {
    try {
      localStorage.setItem(CACHE_KEY, JSON.stringify(settings));
    } catch {
      /* ignore quota / unavailable */
    }
  }

  async function refreshFromServer() {
    try {
      settings = await api.notifications.getSettings();
      persist();
    } catch {
      // Offline / server unreachable — keep the cached copy.
    }
    loaded = true;
  }

  async function flushPending() {
    if (localStorage.getItem(PENDING_KEY) !== '1') return;
    try {
      settings = await api.notifications.putSettings(settings);
      persist();
      localStorage.removeItem(PENDING_KEY);
    } catch {
      // Still offline — leave the pending flag set for the next attempt.
    }
  }

  /**
   * Idempotent. Returns instantly with the cached settings (so the UI never
   * hangs), then reconciles with the server in the background.
   */
  async function init() {
    if (started) {
      return;
    }
    started = true;
    settings = readCache();
    loaded = true;

    if (typeof window !== 'undefined') {
      window.addEventListener('online', () => void flushPending());
    }

    // Background reconcile — push any offline edit first, then pull the
    // authoritative copy. Not awaited, so callers render from cache immediately.
    void (async () => {
      await flushPending();
      await refreshFromServer();
    })();
  }

  /** Persist locally immediately; sync to server when reachable. */
  async function save(next: NotificationSettings) {
    settings = next;
    persist();
    try {
      settings = await api.notifications.putSettings(next);
      persist();
      localStorage.removeItem(PENDING_KEY);
    } catch {
      // Offline — remember to push on reconnect. The local cache already
      // reflects the change so the UI and routines stay consistent.
      try {
        localStorage.setItem(PENDING_KEY, '1');
      } catch {
        /* ignore */
      }
    }
  }

  return {
    get settings() {
      return settings;
    },
    get loaded() {
      return loaded;
    },
    init,
    save,
    refreshFromServer,
    flushPending,
  };
}

export const notificationSettings = createNotificationSettingsStore();
