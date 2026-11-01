// Windows の release で黒い窓を出さない（対応 OS ではないが、害は無い）。
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod askpass;
mod connection;
mod ssh_config;

use std::path::PathBuf;
use std::sync::Arc;

use tauri::{Manager, RunEvent, State};

use askpass::AskpassServer;
use connection::Connections;

struct AppState {
    conns: Connections,
    askpass: Arc<AskpassServer>,
}

/// `~/.ssh/config` のホスト一覧（「このマシン」は含まない。画面側で足す）。
#[tauri::command]
fn list_ssh_hosts() -> Vec<String> {
    std::env::var_os("HOME")
        .map(PathBuf::from)
        .map(|h| ssh_config::list_hosts(&h))
        .unwrap_or_default()
}

/// shj につなぐ。`target` は ssh のホスト名か `@local`。`id` は画面側が決める（イベントの宛先）。
#[tauri::command]
fn connect(
    app: tauri::AppHandle,
    state: State<AppState>,
    id: String,
    target: String,
) -> Result<(), String> {
    state.conns.open(&app, &state.askpass, id, &target)
}

/// プロトコルの 1 メッセージを送る。
#[tauri::command]
fn send(state: State<AppState>, id: String, text: String) -> Result<(), String> {
    state.conns.send(&id, &text)
}

/// 接続を閉じる（待っている ssh の質問も取り消す）。リモートの shj serve は止まらない。
#[tauri::command]
fn disconnect(state: State<AppState>, id: String) {
    state.askpass.cancel_conn(&id);
    state.conns.close(&id);
}

/// ssh の質問（askpass）への答え。`None` は取り消し。
#[tauri::command]
fn askpass_reply(state: State<AppState>, request_id: u64, value: Option<String>) {
    state.askpass.reply(request_id, value);
}

fn main() {
    // ssh が SSH_ASKPASS としてこの実行ファイルを起動したときは、窓を出さず質問を本体に中継する。
    if std::env::var_os(askpass::ENV_SOCK).is_some() {
        let prompt = std::env::args().nth(1).unwrap_or_default();
        std::process::exit(askpass::run_helper(&prompt));
    }

    tauri::Builder::default()
        .setup(|app| {
            let askpass = AskpassServer::start(app.handle().clone())?;
            app.manage(AppState {
                conns: Connections::default(),
                askpass,
            });
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            list_ssh_hosts,
            connect,
            send,
            disconnect,
            askpass_reply
        ])
        .build(tauri::generate_context!())
        .expect("failed to build the shijimi-editor GUI")
        .run(|app, event| {
            if let RunEvent::Exit = event {
                if let Some(s) = app.try_state::<AppState>() {
                    s.conns.close_all();
                    s.askpass.cleanup();
                }
            }
        });
}
