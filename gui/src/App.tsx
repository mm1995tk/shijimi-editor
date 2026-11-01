// 接続の画面（C3）。つなぐ先（このマシンと ~/.ssh/config のホスト）を選んで shj につなぎ、
// ssh の質問（パスワードなど）を窓に出す。編集の画面は D 以降で作る。
import {
  createSignal,
  For,
  onCleanup,
  onMount,
  type JSX,
} from "solid-js";

import { AskpassDialog } from "./AskpassDialog.tsx";
import { LOCAL, type AskpassRequest, type ShellApi } from "./ssh/api.ts";
import { connectShj, type Connection } from "./ssh/connect.ts";
import { tauriShell } from "./ssh/tauri.ts";

type State =
  | { kind: "idle" }
  | { kind: "connecting"; target: string }
  | { kind: "connected"; target: string; conn: Connection }
  | { kind: "failed"; target: string; message: string };

const label = (t: string) => (t === LOCAL ? "このマシン" : t);

function ConnectionStatus(props: { state: State }): JSX.Element {
  return (
    <>
      {(() => {
        const s = props.state;
        switch (s.kind) {
          case "connecting":
            return <p role="status">{label(s.target)} につないでいます…</p>;
          case "connected":
            return (
              <p role="status">
                {label(s.target)} につながりました（shj{" "}
                {s.conn.hello.shj.version}）
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
    </>
  );
}

export function App(props: { api?: ShellApi }): JSX.Element {
  const api = props.api ?? tauriShell;
  const [hosts, setHosts] = createSignal<string[]>([]);
  const [state, setState] = createSignal<State>({ kind: "idle" });
  const [asks, setAsks] = createSignal<AskpassRequest[]>([]);

  // await の後は Owner が無いので、onCleanup は先に登録する
  const offs: (() => void)[] = [];
  onCleanup(() => offs.forEach((f) => f()));
  onMount(async () => {
    setHosts(await api.listSshHosts().catch(() => []));
    offs.push(
      await api.onAskpass((e) => setAsks((a) => [...a, e])),
      await api.onAskpassDone((id) =>
        setAsks((a) => a.filter((x) => x.requestId !== id)),
      ),
    );
  });

  async function connect(target: string) {
    const prev = state();
    if (prev.kind === "connected") void prev.conn.close(); // 前の ssh / attach を残さない
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

  function reply(requestId: number, value: string | null) {
    setAsks((a) => a.filter((x) => x.requestId !== requestId));
    void api.askpassReply(requestId, value);
  }

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
      <ConnectionStatus state={state()} />
      <AskpassDialog ask={asks()[0]} onReply={reply} />
    </main>
  );
}
