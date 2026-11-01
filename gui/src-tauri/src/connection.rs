//! shj への接続。`ssh -T host -- <shj> attach`（リモート）か、`shj attach` の直接起動（このマシン）を
//! 子プロセスとして持ち、その標準入出力を Web 画面（Client の Transport）に中継する。
//! プロトコルは解釈しない（1 行ずつ運ぶだけ）。

use std::collections::HashMap;
use std::io::{BufRead, BufReader, Write};
use std::process::{Child, ChildStdin, Command, Stdio};
use std::sync::{Arc, Mutex};

use serde::Serialize;
use tauri::{AppHandle, Emitter};

use crate::askpass::{self, AskpassServer};

/// 「このマシン」を表す接続先の名前。ssh のホスト名と取り違えないよう、`@` で始める（ホスト名には使えない）。
pub const LOCAL: &str = "@local";

#[derive(Serialize, Clone)]
struct LineEvent {
    id: String,
    line: String,
}

#[derive(Serialize, Clone)]
struct ClosedEvent {
    id: String,
    code: Option<i32>,
    /// ssh の標準エラー出力の最後の数行（つながらなかった理由の表示用）
    stderr: String,
}

struct Conn {
    child: Arc<Mutex<Child>>,
    stdin: Mutex<ChildStdin>,
}

#[derive(Default)]
pub struct Connections {
    map: Arc<Mutex<HashMap<String, Arc<Conn>>>>,
}

/// リモートの shj の場所。`~` はリモートのシェルが展開する。版ごとに分ける（設計書 3 章）。
fn remote_shj() -> String {
    std::env::var("SHIJIMI_REMOTE_SHJ").unwrap_or_else(|_| {
        format!("~/.cache/shijimi-editor/{}/shj", env!("CARGO_PKG_VERSION"))
    })
}

/// ssh の引数に渡してよいホスト名か。`-` で始まるとオプションになり、空白や制御文字は別の引数に化ける。
fn valid_host(h: &str) -> bool {
    !h.is_empty()
        && h.len() <= 255
        && !h.starts_with('-')
        && !h.starts_with('@')
        && !h.chars().any(|c| c.is_whitespace() || c.is_control())
}

fn build_command(target: &str, ask: &AskpassServer, id: &str) -> Result<Command, String> {
    if target == LOCAL {
        let shj = std::env::var_os("SHIJIMI_SHJ").unwrap_or_else(|| "shj".into());
        let mut c = Command::new(shj);
        c.arg("attach");
        return Ok(c);
    }
    if !valid_host(target) {
        return Err(format!("invalid host name: {target:?}"));
    }
    let exe = std::env::current_exe().map_err(|e| e.to_string())?;
    let mut c = Command::new("ssh");
    c.args(["-T", target, "--"])
        .arg(format!("{} attach", remote_shj()))
        .env("SSH_ASKPASS", exe)
        // DISPLAY が無くても、端末が無くても askpass を使わせる（OpenSSH 8.4 以降）
        .env("SSH_ASKPASS_REQUIRE", "force")
        .env(askpass::ENV_SOCK, &ask.sock)
        .env(askpass::ENV_TOKEN, &ask.token)
        .env(askpass::ENV_CONN, id);
    Ok(c)
}

impl Connections {
    pub fn open(
        &self,
        app: &AppHandle,
        ask: &AskpassServer,
        id: String,
        target: &str,
    ) -> Result<(), String> {
        if id.is_empty() || id.len() > 64 {
            return Err("invalid connection id".into());
        }
        if self.map.lock().unwrap().contains_key(&id) {
            return Err("connection id already in use".into());
        }
        let mut cmd = build_command(target, ask, &id)?;
        let mut child = cmd
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
            .map_err(|e| format!("failed to start {:?}: {e}", cmd.get_program()))?;
        let stdin = child.stdin.take().ok_or("no stdin")?;
        let stdout = child.stdout.take().ok_or("no stdout")?;
        let stderr = child.stderr.take().ok_or("no stderr")?;
        let child = Arc::new(Mutex::new(child));
        let conn = Arc::new(Conn {
            child: Arc::clone(&child),
            stdin: Mutex::new(stdin),
        });
        self.map.lock().unwrap().insert(id.clone(), conn);

        // 標準エラーは最後の 20 行だけ覚える（プロトコルには混ぜない）
        let tail: Arc<Mutex<Vec<String>>> = Arc::default();
        let err_thread = {
            let tail = Arc::clone(&tail);
            std::thread::spawn(move || {
                for line in BufReader::new(stderr).split(b'\n').map_while(Result::ok) {
                    let mut t = tail.lock().unwrap();
                    t.push(String::from_utf8_lossy(&line).into_owned());
                    if t.len() > 20 {
                        t.remove(0);
                    }
                }
            })
        };

        let app = app.clone();
        let map = Arc::clone(&self.map);
        std::thread::spawn(move || {
            for line in BufReader::new(stdout).split(b'\n').map_while(Result::ok) {
                let line = String::from_utf8_lossy(&line).into_owned();
                let _ = app.emit(
                    "shj-line",
                    LineEvent {
                        id: id.clone(),
                        line,
                    },
                );
            }
            let _ = err_thread.join();
            let code = child.lock().unwrap().wait().ok().and_then(|s| s.code());
            map.lock().unwrap().remove(&id); // 終わった後の send は失敗させる
            let stderr = tail.lock().unwrap().join("\n");
            let _ = app.emit("shj-closed", ClosedEvent { id, code, stderr });
        });
        Ok(())
    }

    /// 1 メッセージ（末尾の改行は足す）を shj に送る。
    pub fn send(&self, id: &str, text: &str) -> Result<(), String> {
        let conn = self.get(id)?;
        let mut w = conn.stdin.lock().unwrap();
        w.write_all(text.as_bytes())
            .and_then(|_| w.write_all(b"\n"))
            .and_then(|_| w.flush())
            .map_err(|e| e.to_string())
    }

    /// 接続を終わらせる（ssh / attach を殺す。リモートの shj serve は止まらない）。
    pub fn close(&self, id: &str) {
        if let Some(conn) = self.map.lock().unwrap().remove(id) {
            let _ = conn.child.lock().unwrap().kill();
        }
    }

    pub fn close_all(&self) {
        let ids: Vec<String> = self.map.lock().unwrap().keys().cloned().collect();
        for id in ids {
            self.close(&id);
        }
    }

    fn get(&self, id: &str) -> Result<Arc<Conn>, String> {
        self.map
            .lock()
            .unwrap()
            .get(id)
            .cloned()
            .ok_or_else(|| "no such connection".to_string())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn host_validation() {
        assert!(valid_host("web1"));
        assert!(valid_host("user@host.example"));
        assert!(!valid_host("-oProxyCommand=x"));
        assert!(!valid_host("a b"));
        assert!(!valid_host(""));
        assert!(!valid_host(LOCAL));
    }
}
