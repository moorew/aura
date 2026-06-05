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

#[cfg(target_os = "windows")]
fn apply_widget_window_flags(window: &tauri::WebviewWindow) {
    use windows::Win32::UI::WindowsAndMessaging::*;

    let hwnd = window.hwnd().unwrap();

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
