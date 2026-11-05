//! OS の通知（ターミナルの kitty OSC 99）。
//!
//! 公式の通知プラグインはデスクトップで押された・閉じられたを返さないので、`notify-rust` を直接使う。
//! 通知を出したら専用のスレッドで利用者の操作を待ち、結果を `notify-response` のイベントで画面に返す。
//! 押されたときは窓も前に出す（欄を前に出すのは画面側）。
//!
//! OS ごとの違い（調べた結果。macOS と Windows は実機で確かめていない）:
//! - Linux: 押されたは、通知サーバーが `default` アクションに対応するときだけ来る。閉じられたはどのサーバーでも来る。
//! - Windows: 押されたも閉じられたも来る。インストール済みのアプリ（AppUserModelID あり）でないと押されたが戻らないことがある。
//! - macOS: NSUserNotification（Apple は非推奨）。押されたは来るが、閉じられたは取れない（期限切れと区別できない）。

use notify_rust::{Notification, NotificationResponse};
use serde::Serialize;
use std::sync::{Arc, Mutex};
use tauri::{AppHandle, Emitter, Manager};

#[derive(Clone, Serialize)]
struct Response {
    /// 画面が通知を出すときに付けた番号
    key: u64,
    /// `activated`（押された）か `closed`（閉じられた）
    kind: &'static str,
}

/// 通知を出し、押された・閉じられたを待って `notify-response` で返す。出せなかったときはエラー。
/// `key` は画面が欄との対応を引くための番号。
#[tauri::command]
pub fn notify_show(
    app: AppHandle,
    key: u64,
    title: String,
    body: Option<String>,
) -> Result<(), String> {
    let mut n = Notification::new();
    n.summary(&title);
    if let Some(body) = &body {
        n.body(body);
    }
    n.auto_icon();
    // Linux の通知サーバーに、押されたときの既定の動きを持たせる（これが無いと押されたが来ない）
    #[cfg(all(unix, not(target_os = "macos")))]
    n.action("default", "default");
    // 入れたアプリのときだけ AppUserModelID を付ける（開発中の実行ファイルでは付けない）
    #[cfg(windows)]
    {
        let dev = std::env::current_exe()
            .ok()
            .and_then(|p| p.parent().map(|d| d.to_path_buf()))
            .map(|d| d.ends_with("target/debug") || d.ends_with("target/release"))
            .unwrap_or(false);
        if !dev {
            n.app_id(&app.config().identifier);
        }
    }
    #[cfg(target_os = "macos")]
    {
        let _ = notify_rust::set_application(if tauri::is_dev() {
            "com.apple.Terminal"
        } else {
            &app.config().identifier
        });
    }
    let handle = n.show().map_err(|e| e.to_string())?;
    std::thread::spawn(move || {
        let kind = Arc::new(Mutex::new(None));
        let slot = Arc::clone(&kind);
        let _ = handle.wait_for_response(move |r: &NotificationResponse| {
            *slot.lock().unwrap() = match r {
                NotificationResponse::Default | NotificationResponse::Action(_) => {
                    Some("activated")
                }
                NotificationResponse::Reply(_) => None,
                NotificationResponse::Closed(_) => Some("closed"),
            };
        });
        let kind = *kind.lock().unwrap();
        let Some(kind) = kind else { return };
        if kind == "activated" {
            front(&app);
        }
        let _ = app.emit("notify-response", Response { key, kind });
    });
    Ok(())
}

/// 窓を前に出す（最小化を戻し、フォーカスを移す）。
fn front(app: &AppHandle) {
    if let Some(w) = app.get_webview_window("main") {
        let _ = w.unminimize();
        let _ = w.show();
        let _ = w.set_focus();
    }
}
