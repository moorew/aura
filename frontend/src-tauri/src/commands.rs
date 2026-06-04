use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Emitter, Manager};

use crate::sync::SyncStatus;

// ── Task count for taskbar badge ────────────────────────────────────────────

#[tauri::command]
pub async fn get_today_task_count() -> Result<u32, String> {
    // This is called from the frontend after querying the local SQLite.
    // The frontend passes the count back to update the taskbar badge.
    // Actual count is computed client-side from the SQL plugin.
    Ok(0)
}

#[tauri::command]
pub async fn update_taskbar_badge(app: AppHandle, count: u32) -> Result<(), String> {
    // On Windows, we'd update the taskbar overlay icon with the count.
    // On other platforms this is a no-op.
    #[cfg(target_os = "windows")]
    {
        update_windows_badge(&app, count);
    }

    let _ = app;
    let _ = count;
    Ok(())
}

#[cfg(target_os = "windows")]
fn update_windows_badge(_app: &AppHandle, _count: u32) {
    // Windows taskbar badge overlay via ITaskbarList3.
    // This requires COM initialization and the Windows Shell API.
    // The overlay icon is a small PNG rendered at runtime with the count.
    // For now, this is a stub — the full implementation uses:
    //   ITaskbarList3::SetOverlayIcon(hwnd, icon, description)
    // where `icon` is dynamically generated with the count text.
}

// ── Quick add task ──────────────────────────────────────────────────────────

#[derive(Debug, Serialize, Deserialize)]
pub struct QuickTask {
    pub title: String,
    pub planned_date: Option<String>,
}

#[tauri::command]
pub async fn quick_add_task(app: AppHandle, task: QuickTask) -> Result<String, String> {
    // Emit to frontend to handle the actual DB insert via SQL plugin
    app.emit("quick-add-task", &task).map_err(|e| e.to_string())?;

    // Return a generated UUID for the new task
    Ok(uuid::Uuid::new_v4().to_string())
}

// ── Sync commands ───────────────────────────────────────────────────────────

#[tauri::command]
pub async fn trigger_sync(app: AppHandle) -> Result<(), String> {
    crate::sync::trigger_manual_sync(&app).await;
    Ok(())
}

#[tauri::command]
pub async fn get_sync_status() -> Result<SyncStatus, String> {
    Ok(SyncStatus {
        syncing: false,
        last_synced_at: None,
        pending_mutations: 0,
        online: false,
    })
}

// ── Server URL config ───────────────────────────────────────────────────────

#[tauri::command]
pub async fn get_server_url() -> Result<String, String> {
    Ok(std::env::var("SEMPA_SERVER_URL").unwrap_or_default())
}

#[tauri::command]
pub async fn set_server_url(url: String) -> Result<(), String> {
    // Persisted via the store plugin from the frontend side.
    // This command is a fallback for setting the env var at runtime.
    std::env::set_var("SEMPA_SERVER_URL", &url);
    Ok(())
}

// ── Window management ───────────────────────────────────────────────────────

#[tauri::command]
pub async fn create_widget_window(app: AppHandle) -> Result<(), String> {
    crate::windows::create_widget(&app).map_err(|e| e.to_string())
}

#[tauri::command]
pub async fn create_sticky_note(
    app: AppHandle,
    note_id: String,
    x: f64,
    y: f64,
    width: f64,
    height: f64,
) -> Result<(), String> {
    crate::windows::create_sticky(&app, &note_id, x, y, width, height)
        .map_err(|e| e.to_string())
}

#[tauri::command]
pub async fn close_sticky_note(app: AppHandle, note_id: String) -> Result<(), String> {
    let label = format!("sticky-{}", note_id);
    if let Some(win) = app.get_webview_window(&label) {
        win.close().map_err(|e| e.to_string())?;
    }
    Ok(())
}

// ── Sticky note position persistence ────────────────────────────────────────

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StickyPosition {
    pub note_id: String,
    pub x: f64,
    pub y: f64,
    pub width: f64,
    pub height: f64,
}

#[tauri::command]
pub async fn save_sticky_positions(_positions: Vec<StickyPosition>) -> Result<(), String> {
    // Persisted via the store plugin from the frontend.
    Ok(())
}

#[tauri::command]
pub async fn get_sticky_positions() -> Result<Vec<StickyPosition>, String> {
    Ok(vec![])
}
