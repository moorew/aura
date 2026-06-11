use tauri::{
    image::Image,
    menu::{Menu, MenuItem},
    tray::TrayIconBuilder,
    AppHandle, Emitter, Manager,
};

pub fn create_tray(app: &AppHandle) -> Result<(), Box<dyn std::error::Error>> {
    let open = MenuItem::with_id(app, "open", "Open sempa", true, None::<&str>)?;
    let show_widget = MenuItem::with_id(app, "show_widget", "Show widget", true, None::<&str>)?;
    let quick_add = MenuItem::with_id(app, "quick_add", "Quick Add Task", true, None::<&str>)?;
    let sync_now = MenuItem::with_id(app, "sync_now", "Sync Now", true, None::<&str>)?;
    let separator = MenuItem::with_id(app, "sep", "────────────", false, None::<&str>)?;
    let quit = MenuItem::with_id(app, "quit", "Exit", true, None::<&str>)?;

    let menu = Menu::with_items(app, &[&open, &show_widget, &quick_add, &sync_now, &separator, &quit])?;

    let icon = app
        .default_window_icon()
        .cloned()
        .unwrap_or_else(|| Image::from_bytes(include_bytes!("../icons/icon.png")).expect("embedded icon"));

    TrayIconBuilder::with_id("main-tray")
        .icon(icon)
        .menu(&menu)
        .tooltip("sempa")
        .on_menu_event(move |app, event| match event.id().as_ref() {
            "open" => {
                if let Some(win) = app.get_webview_window("main") {
                    let _ = win.show();
                    let _ = win.set_focus();
                }
            }
            "show_widget" => {
                // Spawn (or re-focus) the floating always-on-top desktop widget
                // showing today's tasks at a glance.
                if let Err(e) = crate::windows::create_widget(app) {
                    eprintln!("widget creation failed: {e}");
                }
            }
            "quick_add" => {
                if let Some(win) = app.get_webview_window("main") {
                    let _ = win.show();
                    let _ = win.set_focus();
                    let _ = win.emit("tray-quick-add", ());
                }
            }
            "sync_now" => {
                let handle = app.clone();
                tauri::async_runtime::spawn(async move {
                    crate::sync::trigger_manual_sync(&handle).await;
                });
            }
            "quit" => {
                app.exit(0);
            }
            _ => {}
        })
        .on_tray_icon_event(|tray, event| {
            if let tauri::tray::TrayIconEvent::DoubleClick { .. } = event {
                let app = tray.app_handle();
                if let Some(win) = app.get_webview_window("main") {
                    let _ = win.show();
                    let _ = win.set_focus();
                }
            }
        })
        .build(app)?;

    Ok(())
}
