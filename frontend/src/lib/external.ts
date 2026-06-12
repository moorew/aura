/**
 * Open a URL in the user's default browser, cross-platform.
 *
 * - Tauri desktop: hand off to the OS via the `open_external` command so the
 *   link opens in the system default browser (not inside the app webview).
 * - Capacitor (Android): use the in-app browser.
 * - Web: a plain new tab.
 */
import { isTauri, isCapacitor } from '$lib/platform';

export async function openExternal(url: string): Promise<void> {
    if (!url) return;

    if (isTauri()) {
        try {
            const { invoke } = await import('@tauri-apps/api/core');
            await invoke('open_external', { url });
            return;
        } catch {
            /* fall through to window.open */
        }
    }

    if (isCapacitor()) {
        try {
            const { Browser } = await import('@capacitor/browser');
            await Browser.open({ url });
            return;
        } catch {
            /* fall through */
        }
    }

    window.open(url, '_blank', 'noopener,noreferrer');
}
