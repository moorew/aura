use tauri::{AppHandle, Manager, WebviewUrl, WebviewWindowBuilder};

/// Create the desktop widget window — a compact, always-on-top panel
/// showing today's tasks at a glance. On Windows, this uses WS_EX_TOOLWINDOW
/// and WS_EX_NOACTIVATE to sit above the desktop without stealing focus.
pub fn create_widget(app: &AppHandle) -> Result<(), Box<dyn std::error::Error>> {
    // Check if widget already exists
    if app.get_webview_window("widget").is_some() {
        return Ok(());
    }

    let mut builder = WebviewWindowBuilder::new(
        app,
        "widget",
        WebviewUrl::App("/widget".into()),
    )
    .title("sempa widget")
    .inner_size(320.0, 240.0)
    .resizable(false)
    .decorations(false)
    .always_on_top(true)
    .transparent(true)
    .skip_taskbar(true)
    .visible(true);

    // Position in bottom-right corner
    if let Ok(monitor) = app.primary_monitor() {
        if let Some(monitor) = monitor {
            let size = monitor.size();
            let scale = monitor.scale_factor();
            let x = (size.width as f64 / scale) - 340.0;
            let y = (size.height as f64 / scale) - 280.0;
            builder = builder.position(x, y);
        }
    }

    let _window = builder.build()?;

    // On Windows, apply WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE via platform-specific API
    #[cfg(target_os = "windows")]
    {
        apply_widget_window_flags(&_window);
    }

    Ok(())
}

/// Create the reminder popup — a Granola-style card that floats in the top-right
/// of the desktop, above the app window and OUTSIDE it, so a fired reminder is
/// visible even when Sempa is in the background. It never steals focus
/// (WS_EX_NOACTIVATE) and persists until the user dismisses it. Card contents
/// and the exact window height are driven from the webview via the
/// `reminder:list` event; this just owns the floating, top-right-anchored shell.
pub fn create_reminder_popup(app: &AppHandle) -> Result<(), Box<dyn std::error::Error>> {
    // Already open → ensure it's visible and on top; the webview refreshes its
    // own contents from the latest event.
    if let Some(win) = app.get_webview_window("reminder") {
        let _ = win.show();
        let _ = win.set_always_on_top(true);
        return Ok(());
    }

    let width = 384.0;
    let height = 140.0; // initial; the webview resizes to fit its cards

    let mut builder = WebviewWindowBuilder::new(
        app,
        "reminder",
        WebviewUrl::App("/reminder-popup".into()),
    )
    .title("sempa reminder")
    .inner_size(width, height)
    .resizable(false)
    .decorations(false)
    .always_on_top(true)
    .transparent(true)
    .skip_taskbar(true)
    .visible(true);

    // Anchor to the top-right corner (small margin). Growing the height later
    // extends the window downward, keeping this corner fixed.
    if let Ok(Some(monitor)) = app.primary_monitor() {
        let size = monitor.size();
        let scale = monitor.scale_factor();
        let x = (size.width as f64 / scale) - width - 16.0;
        let y = 16.0;
        builder = builder.position(x, y);
    }

    let _window = builder.build()?;

    #[cfg(target_os = "windows")]
    {
        apply_widget_window_flags(&_window);
    }

    Ok(())
}

#[cfg(target_os = "windows")]
fn apply_widget_window_flags(window: &tauri::WebviewWindow) {
    use windows::Win32::UI::WindowsAndMessaging::*;
    use windows::Win32::Foundation::HWND;

    let hwnd = window.hwnd().unwrap();
    let hwnd = HWND(hwnd.0 as *mut std::ffi::c_void);

    unsafe {
        let ex_style = GetWindowLongW(hwnd, GWL_EXSTYLE);
        SetWindowLongW(
            hwnd,
            GWL_EXSTYLE,
            ex_style | WS_EX_TOOLWINDOW.0 as i32 | WS_EX_NOACTIVATE.0 as i32,
        );
    }
}

/// Create a sticky note window — a borderless, draggable post-it.
pub fn create_sticky(
    app: &AppHandle,
    note_id: &str,
    x: f64,
    y: f64,
    width: f64,
    height: f64,
) -> Result<(), Box<dyn std::error::Error>> {
    let label = format!("sticky-{}", note_id);

    if app.get_webview_window(&label).is_some() {
        return Ok(());
    }

    let url = format!("/sticky?id={}", note_id);

    WebviewWindowBuilder::new(app, &label, WebviewUrl::App(url.into()))
        .title("sempa note")
        .inner_size(width, height)
        .min_inner_size(180.0, 120.0)
        .position(x, y)
        .resizable(true)
        .decorations(false)
        .always_on_top(true)
        .transparent(false)
        .skip_taskbar(true)
        .visible(true)
        .build()?;

    Ok(())
}
