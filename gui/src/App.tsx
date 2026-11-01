// 接続の画面（C3）。つなぐ先（このマシンと ~/.ssh/config のホスト）を選んで shj につなぎ、
// ssh の質問（パスワードなど）を窓に出す。編集の画面は D 以降で作る。
import {
  createSignal,
  For,
  onCleanup,
  onMount,
  Show,
  type JSX,
} from "solid-js";

import { LOCAL, type AskpassRequest, type ShellApi } from "./ssh/api.ts";
import { connectShj, type Connection } from "./ssh/connect.ts";
import { tauriShell } from "./ssh/tauri.ts";

type State =
  | { kind: "idle" }
  | { kind: "connecting"; target: string }
  | { kind: "connected"; target: string; conn: Connection }
  | { kind: "failed"; target: string; message: string };

export function App(props: { api?: ShellApi }): JSX.Element {
  const api = props.api ?? tauriShell;
  const [hosts, setHosts] = createSignal<string[]>([]);
  const [state, setState] = createSignal<State>({ kind: "idle" });
  const [asks, setAsks] = createSignal<AskpassRequest[]>([]);
  const [answer, setAnswer] = createSignal("");

  onMount(async () => {
    setHosts(await api.listSshHosts().catch(() => []));
    const offs = [
      await api.onAskpass((e) => setAsks((a) => [...a, e])),
      await api.onAskpassDone((id) =>
        setAsks((a) => a.filter((x) => x.requestId !== id)),
      ),
    ];
    onCleanup(() => offs.forEach((f) => f()));
  });

  async function connect(target: string) {
    setState({ kind: "connecting", target });
    try {
      const conn = await connectShj(api, target);
      conn.client.onClose(() =>
        setState((s) =>
          s.kind === "connected" && s.conn === conn ? { kind: "idle" } : s,
        ),
      );
      setState({ kind: "connected", target, conn });
    } catch (e) {
      setState({
        kind: "failed",
        target,
        message: e instanceof Error ? e.message : String(e),
      });
    }
  }

  function reply(value: string | null) {
    const ask = asks()[0];
    if (!ask) return;
    setAsks((a) => a.slice(1));
    setAnswer("");
    void api.askpassReply(ask.requestId, value);
  }

  const label = (t: string) => (t === LOCAL ? "このマシン" : t);
  // ホスト鍵の確認（yes/no）は見える文字で打たせる。それ以外（パスワード・認証コード）は伏せる
  const isSecret = (prompt: string) => !/\(yes\/no|\[fingerprint\]/i.test(prompt);

  return (
    <main aria-label="shijimi-editor">
      <h1>つなぐ先</h1>
      <ul>
        <For each={[LOCAL, ...hosts()]}>
          {(t) => (
            <li>
              <button
                disabled={state().kind === "connecting"}
                onClick={() => void connect(t)}
              >
                {label(t)}
              </button>
            </li>
          )}
        </For>
      </ul>
      {(() => {
        const s = state();
        switch (s.kind) {
          case "connecting":
            return <p role="status">{label(s.target)} につないでいます…</p>;
          case "connected":
            return (
              <p role="status">
                {label(s.target)} につながりました（shj {s.conn.hello.shj.version}）
              </p>
            );
          case "failed":
            return (
              <pre role="alert">
                {label(s.target)} につなげませんでした{"\n"}
                {s.message}
              </pre>
            );
          default:
            return null;
        }
      })()}
      <Show when={asks()[0]}>
        {(ask) => (
          <dialog open aria-modal="true" aria-label="ssh の質問">
            <form
              method="dialog"
              onSubmit={(e) => {
                e.preventDefault();
                reply(answer());
              }}
            >
              <p>{ask().prompt}</p>
              <input
                type={isSecret(ask().prompt) ? "password" : "text"}
                value={answer()}
                onInput={(e) => setAnswer(e.currentTarget.value)}
                autofocus
              />
              <button type="submit">OK</button>
              <button type="button" onClick={() => reply(null)}>
                取り消し
              </button>
            </form>
          </dialog>
        )}
      </Show>
    </main>
  );
}
