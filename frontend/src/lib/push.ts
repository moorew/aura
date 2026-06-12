/**
 * Push notification registration for Android (Capacitor).
 * Requests permission, gets the FCM token, and registers it with the backend.
 * Falls back silently on web where PushNotifications plugin is unavailable.
 */

import { api } from './api';
import { NOTIFICATION_SOUNDS } from './sounds';

interface PushPlugin {
  requestPermissions(): Promise<{ receive: string }>;
  register(): Promise<void>;
  addListener(event: string, cb: (data: any) => void): Promise<any>;
  createChannel?(channel: {
    id: string; name: string; description?: string; sound?: string;
    importance?: number; visibility?: number; vibration?: boolean;
  }): Promise<void>;
}

/**
 * Create one Android notification channel per sound. A channel's sound +
 * importance are IMMUTABLE once created, so each choice needs its own channel
 * bound to res/raw/<id>.mp3 — the backend targets these same channel IDs in the
 * FCM payload. The channel id is versioned (must match localReminders.ts and the
 * backend fcm.go) so the corrected sound reference replaces the old broken
 * channel on existing installs. The sound must be the FULL filename, with
 * extension, as the plugin resolves it relative to res/raw.
 */
const CHANNEL_VERSION = 'v2';
async function ensureSoundChannels(plugin: PushPlugin) {
  if (!plugin.createChannel) return;
  for (const snd of NOTIFICATION_SOUNDS) {
    try {
      await plugin.createChannel({
        id: `rem_${snd.id}_${CHANNEL_VERSION}`,
        name: `Reminder — ${snd.label}`,
        description: 'Sempa task reminders',
        sound: `${snd.id}.mp3`, // res/raw filename WITH extension — required to resolve
        importance: 5, // IMPORTANCE_HIGH — heads-up + sound
        visibility: 1,
        vibration: true,
      });
    } catch {
      /* channel already exists or plugin unsupported */
    }
  }
}

function getPlugin(): PushPlugin | null {
  try {
    const cap = (window as any).Capacitor;
    if (cap?.Plugins?.PushNotifications) {
      return cap.Plugins.PushNotifications as PushPlugin;
    }
  } catch {}
  return null;
}

let initialized = false;

/**
 * Call once on app startup (after auth is confirmed).
 * Requests notification permission, gets FCM token, sends to backend.
 */
export async function initPushNotifications() {
  if (initialized) return;
  const plugin = getPlugin();
  if (!plugin) return;

  try {
    const perm = await plugin.requestPermissions();
    if (perm.receive !== 'granted') return;

    // Register a channel per sound so the user's choice plays natively.
    await ensureSoundChannels(plugin);

    await plugin.addListener('registration', async (token: { value: string }) => {
      try {
        await api.devices.register(token.value, 'android');
      } catch (e) {
        console.warn('Failed to register push token:', e);
      }
    });

    await plugin.addListener('registrationError', (err: any) => {
      console.warn('Push registration error:', err);
    });

    await plugin.register();
    initialized = true;
  } catch (e) {
    console.warn('Push notifications init failed:', e);
  }
}
