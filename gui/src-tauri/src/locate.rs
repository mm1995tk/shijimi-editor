//! shj の探し方。
//!
//! - **このマシン**：Finder や Dock から起動した macOS のアプリは、ログインシェルの PATH を引き継がない
//!   （`/usr/bin:/bin:/usr/sbin:/sbin` だけ）。そこで、ログインシェルに PATH を聞き（時間切れ付き）、
//!   GUI の PATH とよく使う場所（[`COMMON_DIRS`]）を足した PATH で `shj` を探す。`shj attach` が起動する
//!   `shj serve` の中のターミナルも同じ PATH を引き継ぐよう、子プロセスの PATH もこれにする。
//! - **ssh 先**：ssh が動かすコマンドのシェル（非対話・非ログイン）の PATH には、Nix のプロファイルなどが
//!   入っていないことがある。GUI の版のキャッシュが無ければ、PATH とよく使う場所の `shj` を使う（[`remote_attach_script`]）。

use std::ffi::{OsStr, OsString};
use std::io::Read;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::sync::{mpsc, OnceLock};
use std::time::Duration;

/// PATH に無くても見に行く場所。`$HOME` と `$USER` はそれぞれの場所で置き換える。
/// Nix（home-manager・nix-darwin・NixOS・単独の nix profile）、Homebrew、`go install`、`~/.local/bin`。
pub const COMMON_DIRS: &[&str] = &[
    "$HOME/.nix-profile/bin",
    "/etc/profiles/per-user/$USER/bin",
    "/run/current-system/sw/bin",
    "/nix/var/nix/profiles/default/bin",
    "/opt/homebrew/bin",
    "/usr/local/bin",
    "$HOME/.local/bin",
    "$HOME/go/bin",
];

/// ログインシェルの答えを待つ上限。プロファイルが重い・止まるときは諦めて、よく使う場所だけで探す。
const LOGIN_SHELL_TIMEOUT: Duration = Duration::from_secs(5);
const PATH_MARKER: &str = "@@shj-path@@";

/// ログインシェルの PATH（取れなければ None）。初めの 1 回だけ聞いて覚える。
fn login_shell_path() -> Option<&'static OsString> {
    static CACHE: OnceLock<Option<OsString>> = OnceLock::new();
    CACHE.get_or_init(ask_login_shell_path).as_ref()
}

/// 起動のときに、別スレッドでログインシェルに PATH を聞いておく（「このマシン」につなぐときに待たないように）。
pub fn warm_up() {
    std::thread::spawn(|| {
        let _ = login_shell_path();
    });
}

fn ask_login_shell_path() -> Option<OsString> {
    let shell = std::env::var_os("SHELL").filter(|s| Path::new(s).is_absolute())?;
    // -i も付ける（VS Code と同じ）。nix の設定を ~/.zshrc などに書いている人もいるため。
    // 標準入力は閉じ、プロファイルが出す文字は印で切り分ける。
    let mut child = Command::new(shell)
        .args(["-l", "-i", "-c"])
        .arg(format!("printf '{PATH_MARKER}%s{PATH_MARKER}' \"$PATH\""))
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::null())
        .spawn()
        .ok()?;
    let mut stdout = child.stdout.take()?;
    let (tx, rx) = mpsc::channel();
    std::thread::spawn(move || {
        let mut buf = Vec::new();
        let _ = stdout.read_to_end(&mut buf);
        let _ = tx.send(buf);
    });
    let out = rx.recv_timeout(LOGIN_SHELL_TIMEOUT).ok();
    let _ = child.kill();
    let _ = child.wait();
    parse_marked_path(&String::from_utf8_lossy(&out?)).map(OsString::from)
}

/// `…@@shj-path@@<PATH>@@shj-path@@…` から PATH を取り出す（プロファイルの出力が前後にあってもよい）。
fn parse_marked_path(out: &str) -> Option<String> {
    let start = out.rfind(PATH_MARKER)?;
    let before = &out[..start];
    let open = before.rfind(PATH_MARKER)?;
    let path = &before[open + PATH_MARKER.len()..];
    (!path.is_empty()).then(|| path.to_string())
}

/// [`COMMON_DIRS`] の `$HOME` と `$USER` を置き換える。分からないものを含む場所は外す。
fn expand_common_dirs(home: Option<&str>, user: Option<&str>) -> Vec<PathBuf> {
    COMMON_DIRS
        .iter()
        .filter_map(|d| {
            let mut s = d.to_string();
            for (var, val) in [("$HOME", home), ("$USER", user)] {
                if s.contains(var) {
                    s = s.replace(var, val.filter(|v| !v.is_empty())?);
                }
            }
            Some(PathBuf::from(s))
        })
        .collect()
}

/// 前から順に、重なりを除いてつなげた PATH。
fn merge_paths<'a>(parts: impl IntoIterator<Item = &'a OsStr>, extra: &[PathBuf]) -> OsString {
    let mut dirs: Vec<PathBuf> = Vec::new();
    let all = parts
        .into_iter()
        .flat_map(|p| std::env::split_paths(p).collect::<Vec<_>>())
        .chain(extra.iter().cloned());
    for d in all {
        if !d.as_os_str().is_empty() && !dirs.contains(&d) {
            dirs.push(d);
        }
    }
    std::env::join_paths(dirs).unwrap_or_default()
}

/// PATH の中から、実行できる name を探す。
fn find_in_path(path: &OsStr, name: &str) -> Option<PathBuf> {
    std::env::split_paths(path)
        .map(|d| d.join(name))
        .find(|p| is_executable(p))
}

#[cfg(unix)]
fn is_executable(p: &Path) -> bool {
    use std::os::unix::fs::PermissionsExt;
    std::fs::metadata(p)
        .map(|m| m.is_file() && m.permissions().mode() & 0o111 != 0)
        .unwrap_or(false)
}

#[cfg(not(unix))]
fn is_executable(p: &Path) -> bool {
    p.is_file()
}

/// 「このマシン」の shj を起動するコマンド（引数はまだ付けない）。
/// `SHIJIMI_SHJ` があればそれを使う。無ければ、ログインシェルの PATH・GUI の PATH・よく使う場所の順に探す。
/// 見つからなければ `shj` のまま（起動に失敗し、画面は「つなげませんでした」を出す）。
pub fn local_shj_command() -> Command {
    let home = std::env::var("HOME").ok();
    let user = std::env::var("USER").ok();
    let gui_path = std::env::var_os("PATH");
    let path = merge_paths(
        login_shell_path()
            .map(OsString::as_os_str)
            .into_iter()
            .chain(gui_path.as_deref()),
        &expand_common_dirs(home.as_deref(), user.as_deref()),
    );
    let shj = std::env::var_os("SHIJIMI_SHJ")
        .or_else(|| find_in_path(&path, "shj").map(PathBuf::into_os_string))
        .unwrap_or_else(|| "shj".into());
    let mut c = Command::new(shj);
    c.env("PATH", path);
    c
}

/// ssh 先で動かす `shj attach`（POSIX sh のスクリプト。単一引用符は含まない）。
/// GUI の版のキャッシュ（`cache`、`$HOME` からの相対）を先に見て、無ければ PATH、次によく使う場所の `shj` を使う。
/// どれも無ければ `exit_missing` で終わる（画面は入れるかを聞く。設計書 3 章）。
/// PATH などで見つかった shj の版が GUI と違っても、そのまま使う（版が違うことは `session.hello` で分かる）。
pub fn remote_attach_script(cache_rel: &str, exit_missing: i32) -> String {
    let dirs: Vec<String> = COMMON_DIRS
        .iter()
        .map(|d| format!("\"{}\"", d.replace("$USER", "$u")))
        .collect();
    format!(
        "s=\"$HOME/{cache_rel}/shj\"; \
         [ -x \"$s\" ] || s=$(command -v shj 2>/dev/null) || s=; \
         if [ -z \"$s\" ]; then u=${{USER:-$(id -un)}}; \
         for d in {dirs}; do if [ -x \"$d/shj\" ]; then s=\"$d/shj\"; break; fi; done; fi; \
         [ -n \"$s\" ] || exit {exit_missing}; exec \"$s\" attach",
        dirs = dirs.join(" "),
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn marked_path_ignores_profile_noise() {
        assert_eq!(
            parse_marked_path("Welcome!\n@@shj-path@@/a/bin:/b/bin@@shj-path@@\nbye\n").as_deref(),
            Some("/a/bin:/b/bin")
        );
        assert_eq!(parse_marked_path("/a/bin:/b/bin"), None);
        assert_eq!(parse_marked_path("@@shj-path@@@@shj-path@@"), None);
        assert_eq!(parse_marked_path("@@shj-path@@/a"), None);
    }

    #[test]
    fn common_dirs_cover_nix_and_homebrew() {
        let d = expand_common_dirs(Some("/Users/me"), Some("me"));
        for want in [
            "/Users/me/.nix-profile/bin",
            "/etc/profiles/per-user/me/bin",
            "/run/current-system/sw/bin",
            "/opt/homebrew/bin",
            "/usr/local/bin",
            "/Users/me/.local/bin",
        ] {
            assert!(d.contains(&PathBuf::from(want)), "{want} missing");
        }
        // HOME や USER が分からなければ、その場所は外す（`/.nix-profile/bin` などにしない）
        let d = expand_common_dirs(None, Some(""));
        assert!(d.iter().all(|p| !p.to_string_lossy().contains('$')));
        assert!(!d.contains(&PathBuf::from("/.nix-profile/bin")));
        assert!(!d.contains(&PathBuf::from("/etc/profiles/per-user//bin")));
        assert!(d.contains(&PathBuf::from("/run/current-system/sw/bin")));
    }

    #[test]
    fn merge_keeps_order_and_drops_duplicates() {
        let p = merge_paths(
            [
                OsStr::new("/login/bin:/usr/bin"),
                OsStr::new("/usr/bin:/bin"),
            ],
            &[PathBuf::from("/login/bin"), PathBuf::from("/extra")],
        );
        assert_eq!(p, OsString::from("/login/bin:/usr/bin:/bin:/extra"));
    }

    #[cfg(unix)]
    #[test]
    fn finds_only_executables() {
        use std::os::unix::fs::PermissionsExt;
        let dir = std::env::temp_dir().join(format!("shj-locate-{}", std::process::id()));
        let a = dir.join("a");
        let b = dir.join("b");
        std::fs::create_dir_all(&a).unwrap();
        std::fs::create_dir_all(&b).unwrap();
        std::fs::write(a.join("shj"), "").unwrap(); // 実行できない
        std::fs::write(b.join("shj"), "").unwrap();
        std::fs::set_permissions(b.join("shj"), std::fs::Permissions::from_mode(0o755)).unwrap();
        let path = std::env::join_paths([&a, &b]).unwrap();
        assert_eq!(find_in_path(&path, "shj"), Some(b.join("shj")));
        assert_eq!(find_in_path(&path, "nope"), None);
        std::fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn remote_script_prefers_the_cache_then_path_then_common_dirs() {
        let s = remote_attach_script(".cache/shijimi-editor/1.2.3", 97);
        // sh -c '…' で包むので単一引用符は使えない。fish の単一引用符の中で意味を持つ \ も避ける
        assert!(!s.contains('\'') && !s.contains('\\'));
        let cache = s.find("$HOME/.cache/shijimi-editor/1.2.3/shj").unwrap();
        let path = s.find("command -v shj").unwrap();
        let nix = s.find("\"$HOME/.nix-profile/bin\"").unwrap();
        assert!(cache < path && path < nix);
        assert!(s.contains("\"/etc/profiles/per-user/$u/bin\""));
        assert!(s.contains("/run/current-system/sw/bin"));
        assert!(s.contains("exit 97"));
        assert!(s.ends_with("exec \"$s\" attach"));
    }

    /// 書いたスクリプトを本物の sh で動かす（キャッシュ・PATH・よく使う場所・無いとき）。
    #[cfg(unix)]
    #[test]
    fn remote_script_runs_in_sh() {
        use std::os::unix::fs::PermissionsExt;
        let root = std::env::temp_dir().join(format!("shj-remote-{}", std::process::id()));
        let home = root.join("home");
        let fake = |p: &Path, tag: &str| {
            std::fs::create_dir_all(p.parent().unwrap()).unwrap();
            std::fs::write(p, format!("#!/bin/sh\necho {tag} \"$1\"\n")).unwrap();
            std::fs::set_permissions(p, std::fs::Permissions::from_mode(0o755)).unwrap();
        };
        let script = remote_attach_script(".cache/shijimi-editor/1.2.3", 97);
        let run = |path: &str| {
            Command::new("/bin/sh")
                .arg("-c")
                .arg(&script)
                .env_clear()
                .env("HOME", &home)
                .env("USER", "nobody-shj-test")
                .env("PATH", path)
                .output()
                .unwrap()
        };
        let sys = "/usr/bin:/bin";

        // どこにも無い
        let out = run(sys);
        assert_eq!(out.status.code(), Some(97));

        // よく使う場所（~/.nix-profile/bin）
        fake(&home.join(".nix-profile/bin/shj"), "nix");
        let out = run(sys);
        assert_eq!(String::from_utf8_lossy(&out.stdout), "nix attach\n");

        // PATH にあれば、よく使う場所より先
        let onpath = root.join("onpath");
        fake(&onpath.join("shj"), "path");
        let out = run(&format!("{}:{sys}", onpath.display()));
        assert_eq!(String::from_utf8_lossy(&out.stdout), "path attach\n");

        // GUI の版のキャッシュが一番先
        fake(&home.join(".cache/shijimi-editor/1.2.3/shj"), "cache");
        let out = run(&format!("{}:{sys}", onpath.display()));
        assert_eq!(String::from_utf8_lossy(&out.stdout), "cache attach\n");

        std::fs::remove_dir_all(&root).unwrap();
    }
}
