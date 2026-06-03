/**
 * Push notification registration for Android (Capacitor).
 * Requests permission, gets the FCM token, and registers it with the backend.
 * Falls back silently on web where PushNotifications plugin is unavailable.
 */

import { api } from './api';

interface PushPlugin {
  requestPermissions(): Promise<{ receive: string }>;
  register(): Promise<void>;
  addListener(event: string, cb: (data: any) => void): Promise<any>;
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
