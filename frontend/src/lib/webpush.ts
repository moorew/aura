/**
 * Web Push (VAPID) registration for browser / PWA — Windows & Android Chrome/Edge.
 *
 * This is the "standard WebPush" path: it registers the service worker (sw.js),
 * requests notification permission, subscribes via the Push API using the
 * backend's VAPID public key, and stores the subscription server-side.
 *
 * Native shells use their own channels and are skipped here:
 *   • Capacitor Android → FCM (see push.ts)
 *   • Tauri desktop     → @tauri-apps/plugin-notification (see routines store)
 */

import { api } from './api';
import { isTauri, isCapacitor } from './platform';

/** True when this environment can do W3C Web Push (a real browser/PWA). */
export function isWebPushSupported(): boolean {
  return (
    typeof window !== 'undefined' &&
    'serviceWorker' in navigator &&
    'PushManager' in window &&
    'Notification' in window &&
    !isTauri() &&
    !isCapacitor()
  );
}

/** Register the push service worker (no-op on unsupported platforms). */
export async function registerServiceWorker(): Promise<ServiceWorkerRegistration | null> {
  if (!isWebPushSupported()) return null;
  try {
    return await navigator.serviceWorker.register('/sw.js');
  } catch (e) {
    console.warn('SW registration failed:', e);
    return null;
  }
}

export function notificationPermission(): NotificationPermission | 'unsupported' {
  if (!isWebPushSupported()) return 'unsupported';
  return Notification.permission;
}

export async function isWebPushSubscribed(): Promise<boolean> {
  if (!isWebPushSupported()) return false;
  const reg = await navigator.serviceWorker.getRegistration();
  const sub = await reg?.pushManager.getSubscription();
  return !!sub;
}

/**
 * Request permission, subscribe to push, and register the subscription with the
 * backend. Returns an error code the settings UI can surface.
 */
export async function enableWebPush(): Promise<{ ok: boolean; error?: string }> {
  if (!isWebPushSupported()) return { ok: false, error: 'unsupported' };

  const perm = await Notification.requestPermission();
  if (perm !== 'granted') return { ok: false, error: 'denied' };

  const reg = (await registerServiceWorker()) ?? (await navigator.serviceWorker.ready);
  await navigator.serviceWorker.ready;

  let key: string;
  try {
    ({ key } = await api.notifications.vapidPublicKey());
  } catch {
    return { ok: false, error: 'no-server' };
  }
  if (!key) return { ok: false, error: 'no-vapid-key' };

  let sub = await reg.pushManager.getSubscription();
  if (!sub) {
    sub = await reg.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(key) as BufferSource,
    });
  }

  const json = sub.toJSON();
  if (!json.keys?.p256dh || !json.keys?.auth) return { ok: false, error: 'bad-subscription' };

  await api.notifications.subscribeWebPush({
    endpoint: sub.endpoint,
    keys: { p256dh: json.keys.p256dh, auth: json.keys.auth },
    platform: 'web',
  });
  return { ok: true };
}

/** Unsubscribe from push and remove the subscription server-side. */
export async function disableWebPush(): Promise<void> {
  if (!('serviceWorker' in navigator)) return;
  const reg = await navigator.serviceWorker.getRegistration();
  const sub = await reg?.pushManager.getSubscription();
  if (!sub) return;
  await api.notifications.unsubscribeWebPush(sub.endpoint).catch(() => {});
  await sub.unsubscribe().catch(() => {});
}

/**
 * Bridge the service worker's deep-link request to the SPA router. The SW posts
 * { type:'navigate', url } when a notification body is clicked; we navigate in
 * place rather than reloading.
 */
export function listenForPushNavigation(navigate: (url: string) => void): void {
  if (!isWebPushSupported()) return;
  navigator.serviceWorker.addEventListener('message', (event) => {
    const data = event.data;
    if (data && data.type === 'navigate' && typeof data.url === 'string') {
      navigate(data.url);
    }
  });
}

function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
  const raw = atob(base64);
  const out = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i);
  return out;
}
