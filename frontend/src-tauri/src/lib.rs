mod commands;
mod db;
mod sync;
mod tray;
mod windows;

use tauri::Manager;
use std::io::Write;

/// Write a line to a startup log file in a location that's always writable,
/// so a launch failure is never silent (Windows release builds have no console).
/// Writes to the OS temp dir, which exists and is writable on every platform.
fn startup_log(msg: &str) {
    let path = std::env::temp_dir().join("sempa-startup.log");
    if let Ok(mut f) = std::fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open(&path)
    {
        let _ = writeln!(f, "[sempa] {msg}");
    }
    // Also emit to stderr for dev/console builds.
    eprintln!("[sempa] {msg}");
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    // Install a panic hook before anything else so any startup panic (e.g. a
    // bad plugin config, a missing WebView2 runtime) is recorded to disk rather
    // than vanishing silently on a windowed Windows build.
    std::panic::set_hook(Box::new(|info| {
        startup_log(&format!("PANIC: {info}"));
    }));

    startup_log("starting up");

    let result = tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_notification::init())
        .plugin(
            tauri_plugin_sql::Builder::new()
                .add_migrations("sqlite:sempa.db", db::get_migrations())
                .build(),
        )
        .plugin(tauri_plugin_autostart::init(
            tauri_plugin_autostart::MacosLauncher::LaunchAgent,
            Some(vec!["--minimized"]),
        ))
        .plugin(tauri_plugin_store::Builder::new().build())
        .setup(|app| {
            startup_log("setup: begin");

            // Initialize the system tray. A tray failure must not take the
            // whole app down — log it and continue so the window still opens.
            if let Err(e) = tray::create_tray(app.handle()) {
                startup_log(&format!("setup: tray creation failed (non-fatal): {e}"));
            }

            // Run database migrations on startup
            let app_handle = app.handle().clone();
            tauri::async_runtime::spawn(async move {
                if let Err(e) = db::run_migrations(&app_handle).await {
                    eprintln!("Migration error: {e}");
                }
            });

            // Start the background sync engine
            let app_handle = app.handle().clone();
            tauri::async_runtime::spawn(async move {
                sync::start_sync_loop(app_handle).await;
            });

            // DevTools open automatically in debug builds; release builds ship
            // without them. (A temporary release auto-open was used to diagnose
            // the sync issue — removed now that it's fixed.)
            #[cfg(debug_assertions)]
            if let Some(win) = app.get_webview_window("main") {
                win.open_devtools();
            }

            // Check if launched with --minimized flag (startup boot)
            let minimized = std::env::args().any(|a| a == "--minimized");
            if minimized {
                if let Some(win) = app.get_webview_window("main") {
                    let _ = win.hide();
                }
            }

            startup_log("setup: complete");
            Ok(())
        })
        .on_window_event(|window, event| {
            // Minimize to tray instead of closing
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                if window.label() == "main" {
                    api.prevent_close();
                    let _ = window.hide();
                }
            }
        })
        .invoke_handler(tauri::generate_handler![
            commands::get_today_task_count,
            commands::quick_add_task,
            commands::trigger_sync,
            commands::get_sync_status,
            commands::get_server_url,
            commands::set_server_url,
            commands::create_widget_window,
            commands::create_sticky_note,
            commands::close_sticky_note,
            commands::save_sticky_positions,
            commands::get_sticky_positions,
            commands::update_taskbar_badge,
        ])
        .run(tauri::generate_context!());

    if let Err(e) = result {
        startup_log(&format!("FATAL: tauri runtime exited with error: {e}"));
        std::process::exit(1);
    }
}
