/**
 * Sempa service worker — Web Push delivery.
 *
 * Handles the two events that drive native OS notifications in the browser/PWA:
 *   • 'push'             → render the notification card (with quick actions)
 *   • 'notificationclick' → run an action ('done'/'snooze') or deep-link the app
 *
 * The backend (internal/notify/dispatch.go) sends a JSON payload of the shape:
 *   { title, body, url, taskId, tag, type, sound }
 *
 * Action requests reuse the normal REST API with the session cookie
 * (credentials:'include'), so a "Mark done" tap updates the DB immediately even
 * if no app window is open.
 */

const ICON = '/icons/icon-192.png';
const BADGE = '/icons/favicon-32.png';

self.addEventListener('install', () => {
  // Activate this worker as soon as it finishes installing.
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  // Take control of already-open clients without requiring a reload.
  event.waitUntil(self.clients.claim());
});

self.addEventListener('push', (event) => {
  let data = {};
  try {
    data = event.data ? event.data.json() : {};
  } catch {
    data = { title: 'Sempa', body: event.data ? event.data.text() : '' };
  }

  const title = data.title || 'Sempa';
  const isReminder = data.type === 'reminder' && data.taskId;

  const options = {
    body: data.body || '',
    icon: ICON,
    badge: BADGE,
    tag: data.tag || undefined,
    renotify: !!data.tag,
    requireInteraction: isReminder, // keep hard reminders on screen until acted on
    silent: data.sound === false,
    data: {
      url: data.url || '/home',
      taskId: data.taskId || '',
      type: data.type || '',
    },
    actions: isReminder
      ? [
          { action: 'done', title: 'Mark done' },
          { action: 'snooze', title: 'Snooze 1h' },
        ]
      : [],
  };

  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener('notificationclick', (event) => {
  const notification = event.notification;
  const { url, taskId } = notification.data || {};
  notification.close();

  // Quick actions run a background API call and do not open the app.
  if (event.action === 'done' && taskId) {
    event.waitUntil(apiCall(`/api/v1/tasks/${taskId}`, 'PATCH', { status: 'done' }));
    return;
  }
  if (event.action === 'snooze' && taskId) {
    event.waitUntil(apiCall(`/api/v1/tasks/${taskId}/snooze`, 'POST', { minutes: 60 }));
    return;
  }

  // Body click → focus an existing window (navigating it) or open a new one,
  // deep-linking straight to the relevant view.
  event.waitUntil(focusOrOpen(url || '/home'));
});

// Re-subscribe transparently if the browser rotates the push subscription, so
// reminders keep arriving without the user re-enabling them.
self.addEventListener('pushsubscriptionchange', (event) => {
  event.waitUntil(
    (async () => {
      try {
        const res = await fetch('/api/v1/notifications/vapid-public-key', { credentials: 'include' });
        const { key } = await res.json();
        const sub = await self.registration.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: urlBase64ToUint8Array(key),
        });
        const json = sub.toJSON();
        await apiCall('/api/v1/notifications/webpush/subscribe', 'POST', {
          endpoint: sub.endpoint,
          keys: json.keys,
          platform: 'web',
        });
      } catch (e) {
        // Best-effort; the next app launch re-subscribes anyway.
      }
    })(),
  );
});

async function apiCall(path, method, payload) {
  try {
    await fetch(path, {
      method,
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(payload),
    });
  } catch {
    // Swallow — the notification action is best-effort from the SW context.
  }
}

async function focusOrOpen(url) {
  const all = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });
  for (const client of all) {
    if ('focus' in client) {
      await client.focus();
      // Ask the page to client-side navigate (avoids a full reload).
      client.postMessage({ type: 'navigate', url });
      return;
    }
  }
  if (self.clients.openWindow) {
    await self.clients.openWindow(url);
  }
}

function urlBase64ToUint8Array(base64String) {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
  const raw = atob(base64);
  const out = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i);
  return out;
}
