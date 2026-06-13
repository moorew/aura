/**
 * Native OS notifications for the Tauri desktop shell.
 *
 * This is the reliability backbone for desktop reminders — the equivalent of the
 * on-device alarm Android gets from @capacitor/local-notifications. Until now the
 * desktop shell registered the Tauri notification plugin but NEVER called it, so a
 * fired reminder had only two in-WebView surfaces (the in-app banner and the
 * floating Granola card). On Windows both can silently fail to appear, leaving the
 * user with just a sound and no idea which task rang. A real OS toast is the
 * channel Windows users expect, shows even when Sempa is backgrounded, and lands
 * in the Action Center so it isn't lost.
 *
 * Permission is requested lazily on first use (and cached), so we never prompt
 * until there's actually something to notify about.
 */

import { isTauri } from '$lib/platform';

let permissionChecked = false;
let permissionGranted = false;

/** Ensure OS notification permission, requesting it once if needed. */
export async function ensureDesktopNotifyPermission(): Promise<boolean> {
  if (!isTauri()) return false;
  if (permissionChecked) return permissionGranted;
  try {
    const { isPermissionGranted, requestPermission } = await import(
      '@tauri-apps/plugin-notification'
    );
    let granted = await isPermissionGranted();
    if (!granted) granted = (await requestPermission()) === 'granted';
    permissionGranted = granted;
  } catch {
    permissionGranted = false;
  }
  permissionChecked = true;
  return permissionGranted;
}

/**
 * Fire a native OS notification. Best-effort: if the plugin is unavailable or
 * permission was denied, the in-app banner / floating card still cover it.
 */
export async function desktopNotify(title: string, body: string): Promise<void> {
  if (!isTauri()) return;
  try {
    if (!(await ensureDesktopNotifyPermission())) return;
    const { sendNotification } = await import('@tauri-apps/plugin-notification');
    sendNotification({ title, body });
  } catch {
    /* no-op — other reminder surfaces remain */
  }
}
