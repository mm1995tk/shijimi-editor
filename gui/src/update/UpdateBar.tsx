// 自動更新の知らせ（C8）。起動したら 1 度だけ新しい版があるかを殻に聞き、あれば右下に出す。
// 入れ替えは「更新して再起動」を押したときだけ行う（編集中に勝手に起動し直さないため）。
// 自動更新は AppImage と .dmg の版だけが持つ。Nix で作る版では殻がいつも「無い」と答えるので、何も出ない。
import { invoke } from "@tauri-apps/api/core";
import { createSignal, onMount, Show, type JSX } from "solid-js";

import "./update.css";

/** 殻（src-tauri/src/update.rs の UpdateInfo）が返す新しい版。 */
type UpdateInfo = { current: string; version: string; notes: string | null };

export function UpdateBar(): JSX.Element {
  const [info, setInfo] = createSignal<UpdateInfo | null>(null);
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal("");

  onMount(() => {
    invoke<UpdateInfo | null>("update_check").then(setInfo, () => {
      // 調べられないのはネットにつながっていないだけのことが多い。知らせない
    });
  });

  async function install() {
    setBusy(true);
    setError("");
    try {
      // うまくいけば起動し直すので、ここには戻らない
      await invoke("update_install");
    } catch (e) {
      setError(String(e));
      setBusy(false);
    }
  }

  return (
    <Show when={info()}>
      {(u) => (
        <aside class="update-bar" role="status" aria-label="新しい版">
          <p>
            新しい版 {u().version} があります（今は {u().current}）。
          </p>
          <button disabled={busy()} onClick={() => void install()}>
            {busy() ? "更新しています…" : "更新して再起動"}
          </button>{" "}
          <button disabled={busy()} onClick={() => setInfo(null)}>
            あとで
          </button>
          <Show when={error()}>
            <pre role="alert">{error()}</pre>
          </Show>
        </aside>
      )}
    </Show>
  );
}
