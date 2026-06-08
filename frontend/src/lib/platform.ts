/**
 * Platform detection shared across the app.
 *
 * - isTauri     → desktop (Windows/macOS/Linux) Tauri shell
 * - isCapacitor → native Android (Capacitor) shell
 * - hasLocalDb  → either of the above (i.e. a local SQLite DB is available)
 * - web         → plain browser served by the Go backend (neither flag set)
 */

export function isTauri(): boolean {
    return typeof window !== 'undefined' && '__TAURI__' in window;
}

export function isCapacitor(): boolean {
    if (typeof window === 'undefined') return false;
    const cap = (window as { Capacitor?: { isNativePlatform?: () => boolean } }).Capacitor;
    return !!cap?.isNativePlatform?.();
}

export function hasLocalDb(): boolean {
    return isTauri() || isCapacitor();
}
