use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Emitter};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SyncStatus {
    pub syncing: bool,
    pub last_synced_at: Option<String>,
    pub pending_mutations: u32,
    pub online: bool,
}

/// Background sync is owned by the shared TypeScript engine ($lib/sync.ts),
/// which runs on both desktop (Tauri) and Android (Capacitor) so there is a
/// single code path with one outbox/cursor reconciliation policy. It accesses
/// the local SQLite via the JS SQL plugin — the same DB this process opens —
/// and talks to the server over HTTP with the user's Bearer token, which the
/// Rust side does not hold. Running a second loop here would only race it, so
/// this is intentionally a no-op kept for the (now unused) manual-trigger path.
pub async fn start_sync_loop(_app: AppHandle) {}

/// Manual sync trigger from the tray menu / `trigger_sync` command. The actual
/// reconciliation runs in the TypeScript engine; here we just nudge the
/// frontend, which listens and calls sync(). We emit a status ping so any
/// listening UI shows immediate feedback.
pub async fn trigger_manual_sync(app: &AppHandle) {
    let _ = app.emit("sync-trigger", ());
}
