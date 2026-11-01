//! `SSH_ASKPASS` を GUI の窓で受ける。
//!
//! ssh は、パスワード・2 段階認証・ホスト鍵の確認が要るとき、環境変数 `SSH_ASKPASS` の指す実行ファイルを
//! 質問の文を第 1 引数にして起動し、その標準出力を答えとして読む。ここではその実行ファイルを GUI 自身にする
//! （`SHIJIMI_ASKPASS_SOCK` が付いて起動されたら、窓は開かず質問を GUI 本体へ中継して終わる）。
//!
//! 中継は Unix ソケット（自分のユーザーだけが入れるフォルダの中）で、1 回ごとに 1 行の JSON を往復する。
//! 質問の持ち主を確かめるため、接続ごとの使い捨ての合言葉（token）を付ける。

use std::io::{BufRead, BufReader, Write};
use std::os::unix::fs::DirBuilderExt;
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::PathBuf;
use std::sync::mpsc::{channel, RecvTimeoutError, Sender};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Emitter};

pub const ENV_SOCK: &str = "SHIJIMI_ASKPASS_SOCK";
pub const ENV_TOKEN: &str = "SHIJIMI_ASKPASS_TOKEN";
pub const ENV_CONN: &str = "SHIJIMI_ASKPASS_CONN";

/// ユーザーが答えないまま待つ上限。過ぎたら取り消し扱いにする。
const ANSWER_TIMEOUT: Duration = Duration::from_secs(180);

#[derive(Serialize, Deserialize)]
struct Ask {
    token: String,
    conn: String,
    prompt: String,
}

#[derive(Serialize, Deserialize)]
struct Answer {
    ok: bool,
    #[serde(default)]
    value: String,
}

#[derive(Serialize, Clone)]
struct AskEvent {
    #[serde(rename = "requestId")]
    request_id: u64,
    conn: String,
    prompt: String,
}

struct Pending {
    conn: String,
    tx: Sender<Option<String>>,
}

pub struct AskpassServer {
    pub sock: PathBuf,
    pub token: String,
    pending: Arc<Mutex<Vec<(u64, Pending)>>>,
}

/// 0700 のフォルダ。`$XDG_RUNTIME_DIR` が無ければ一時フォルダの下に作る（0700 で、既にあれば失敗させる）。
fn private_dir() -> std::io::Result<PathBuf> {
    let base = std::env::var_os("XDG_RUNTIME_DIR")
        .map(PathBuf::from)
        .filter(|p| p.is_dir())
        .unwrap_or_else(std::env::temp_dir);
    let dir = base.join(format!("shijimi-editor-gui-{}", std::process::id()));
    let _ = std::fs::remove_dir_all(&dir); // 同じ pid の前の残り
    std::fs::DirBuilder::new().mode(0o700).create(&dir)?;
    Ok(dir)
}

pub fn random_token() -> std::io::Result<String> {
    use std::io::Read;
    let mut b = [0u8; 16];
    std::fs::File::open("/dev/urandom")?.read_exact(&mut b)?;
    Ok(b.iter().map(|x| format!("{x:02x}")).collect())
}

impl AskpassServer {
    pub fn start(app: AppHandle) -> std::io::Result<Arc<Self>> {
        let dir = private_dir()?;
        let sock = dir.join("askpass.sock");
        let listener = UnixListener::bind(&sock)?;
        let server = Arc::new(Self {
            sock,
            token: random_token()?,
            pending: Arc::new(Mutex::new(Vec::new())),
        });
        let s = Arc::clone(&server);
        std::thread::spawn(move || {
            let mut next_id: u64 = 1;
            for stream in listener.incoming().flatten() {
                let id = next_id;
                next_id += 1;
                let s = Arc::clone(&s);
                let app = app.clone();
                std::thread::spawn(move || s.handle(stream, id, &app));
            }
        });
        Ok(server)
    }

    fn handle(&self, stream: UnixStream, id: u64, app: &AppHandle) {
        let mut line = String::new();
        let Ok(r) = stream.try_clone() else { return };
        // 質問は 1 行。長すぎる入力は読まない。
        if BufReader::new(r).take_line(&mut line, 64 * 1024).is_err() {
            return;
        }
        let Ok(ask) = serde_json::from_str::<Ask>(&line) else {
            return;
        };
        let answer = if ask.token == self.token {
            let (tx, rx) = channel();
            self.pending.lock().unwrap().push((
                id,
                Pending {
                    conn: ask.conn.clone(),
                    tx,
                },
            ));
            let _ = app.emit(
                "askpass",
                AskEvent {
                    request_id: id,
                    conn: ask.conn,
                    prompt: ask.prompt,
                },
            );
            let v = match rx.recv_timeout(ANSWER_TIMEOUT) {
                Ok(v) => v,
                Err(RecvTimeoutError::Timeout | RecvTimeoutError::Disconnected) => None,
            };
            self.pending.lock().unwrap().retain(|(i, _)| *i != id);
            let _ = app.emit("askpass-done", id);
            Answer {
                ok: v.is_some(),
                value: v.unwrap_or_default(),
            }
        } else {
            Answer {
                ok: false,
                value: String::new(),
            }
        };
        let mut w = stream;
        if let Ok(s) = serde_json::to_string(&answer) {
            let _ = writeln!(w, "{s}");
        }
    }

    /// 窓で答えた（`None` は取り消し）。
    pub fn reply(&self, request_id: u64, value: Option<String>) {
        let mut p = self.pending.lock().unwrap();
        if let Some(pos) = p.iter().position(|(i, _)| *i == request_id) {
            let (_, pend) = p.remove(pos);
            let _ = pend.tx.send(value);
        }
    }

    /// 接続を閉じたとき、その接続の待ちをすべて取り消す。
    pub fn cancel_conn(&self, conn: &str) {
        let mut p = self.pending.lock().unwrap();
        p.retain(|(_, pend)| {
            if pend.conn == conn {
                let _ = pend.tx.send(None);
                false
            } else {
                true
            }
        });
    }
}

/// 1 行を上限つきで読む。
trait TakeLine {
    fn take_line(&mut self, buf: &mut String, max: u64) -> std::io::Result<()>;
}
impl<R: BufRead> TakeLine for R {
    fn take_line(&mut self, buf: &mut String, max: u64) -> std::io::Result<()> {
        use std::io::Read;
        let mut bytes = Vec::new();
        let mut limited = self.by_ref().take(max);
        limited.read_until(b'\n', &mut bytes)?;
        *buf = String::from_utf8_lossy(&bytes).into_owned();
        Ok(())
    }
}

/// `SSH_ASKPASS` として起動されたときの本体。ssh に答えを標準出力で返す。終了コードをそのまま返す。
pub fn run_helper(prompt: &str) -> i32 {
    let (Some(sock), Some(token), Some(conn)) = (
        std::env::var_os(ENV_SOCK),
        std::env::var(ENV_TOKEN).ok(),
        std::env::var(ENV_CONN).ok(),
    ) else {
        return 2;
    };
    let Ok(mut s) = UnixStream::connect(sock) else {
        return 2;
    };
    let ask = Ask {
        token,
        conn,
        prompt: prompt.to_string(),
    };
    let Ok(text) = serde_json::to_string(&ask) else {
        return 2;
    };
    if writeln!(s, "{text}").is_err() {
        return 2;
    }
    let mut line = String::new();
    if BufReader::new(s).take_line(&mut line, 64 * 1024).is_err() {
        return 2;
    }
    match serde_json::from_str::<Answer>(&line) {
        Ok(a) if a.ok => {
            println!("{}", a.value);
            0
        }
        _ => 1, // 取り消し。ssh は認証を諦める
    }
}
