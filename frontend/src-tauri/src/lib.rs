mod commands;
mod db;
mod sync;
mod tray;
mod windows;

use tauri::Manager;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
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
            // Initialize the system tray
            tray::create_tray(app.handle())?;

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

            // Check if launched with --minimized flag (startup boot)
            let minimized = std::env::args().any(|a| a == "--minimized");
            if minimized {
                if let Some(win) = app.get_webview_window("main") {
                    let _ = win.hide();
                }
            }

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
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
