// 接続の画面（C3〜C5）。起動時は、前回ワークスペースがあったマシンにだけ自動でつなぐ（C5）。
// つなぐ先（このマシンと ~/.ssh/config のホスト）を選んで shj につなぎ、
// ssh の質問（パスワードなど）を窓に出す。切れたら Session が自動でつなぎ直し、
// つなぐ先の一覧に接続の状態を出す。編集の画面は D 以降で作る。
import {
  createSignal,
  For,
  onCleanup,
  onMount,
  Show,
  type JSX,
} from "solid-js";

import { AskpassDialog } from "./AskpassDialog.tsx";
import { LOCAL, type AskpassRequest, type ShellApi } from "./ssh/api.ts";
import {
  autoConnectTargets,
  browserStore,
  loadLastTargets,
  recordWorkspaces,
  type KeyValueStore,
} from "./ssh/last.ts";
import { Session, type SessionState } from "./ssh/session.ts";
import { tauriShell } from "./ssh/tauri.ts";

const label = (t: string) => (t === LOCAL ? "このマシン" : t);

/** 一覧に出す 1 行の状態。 */
export function describeState(
  s: SessionState | undefined,
  now: number = Date.now(),
): string {
  if (!s) return "";
  switch (s.kind) {
    case "connecting":
      if (s.retryAt !== undefined) {
        const sec = Math.max(0, Math.ceil((s.retryAt - now) / 1000));
        return `つなげませんでした。${sec} 秒後にもう一度（${s.attempt}/${s.max} 回目）`;
      }
      return s.attempt > 1
        ? `つないでいます…（${s.attempt}/${s.max} 回目）`
        : "つないでいます…";
    case "connected":
      return `つながっています（shj ${s.hello.shj.version}）`;
    case "reconnecting": {
      const n = `${s.attempt}/${s.max} 回目`;
      if (s.retryAt !== undefined) {
        const sec = Math.max(0, Math.ceil((s.retryAt - now) / 1000));
        return `切れました。${sec} 秒後につなぎ直します（${n}）`;
      }
      return `切れました。つなぎ直しています…（${n}）`;
    }
    case "failed":
      return "つながりません";
    case "superseded":
      return "別の GUI に移りました";
    case "closed":
      return "";
  }
}

export function App(props: {
  api?: ShellApi;
  /** 前回の記録の置き場。省くと localStorage */
  store?: KeyValueStore;
}): JSX.Element {
  const api = props.api ?? tauriShell;
  const store = props.store ?? browserStore();
  const [hosts, setHosts] = createSignal<string[]>([]);
  const [sessions, setSessions] = createSignal<Record<string, Session>>({});
  const [states, setStates] = createSignal<Record<string, SessionState>>({});
  const offStates = new Map<string, () => void>();
  const [asks, setAsks] = createSignal<AskpassRequest[]>([]);
  // 「あと何秒」を動かす
  const [now, setNow] = createSignal(Date.now());
  const tick = setInterval(() => setNow(Date.now()), 1000);

  // await の後は Owner が無いので、onCleanup は先に登録する
  const offs: (() => void)[] = [];
  onCleanup(() => {
    clearInterval(tick);
    offs.forEach((f) => f());
    for (const s of Object.values(sessions())) void s.close();
  });
  onMount(async () => {
    const found = await api.listSshHosts().catch(() => []);
    setHosts(found);
    offs.push(
      await api.onAskpass((e) => setAsks((a) => [...a, e])),
      await api.onAskpassDone((id) =>
        setAsks((a) => a.filter((x) => x.requestId !== id)),
      ),
    );
    // 質問の窓口を用意してから、前回ワークスペースがあったマシンにだけつなぐ
    for (const t of autoConnectTargets(loadLastTargets(store), [
      LOCAL,
      ...found,
    ])) {
      connect(t);
    }
  });

  function connect(target: string) {
    const old = sessions()[target];
    if (old) {
      if (old.state.kind === "failed" || old.state.kind === "superseded") {
        old.retry();
        return;
      }
      return; // つないでいる最中、またはつながっている
    }
    const s = new Session(api, target);
    const off = s.onState((st) => setStates((m) => ({ ...m, [target]: st })));
    const offWs = s.onWorkspaces((ws) =>
      recordWorkspaces(store, target, ws.length),
    );
    offStates.set(target, () => {
      off();
      offWs();
    });
    setSessions((m) => ({ ...m, [target]: s }));
    setStates((m) => ({ ...m, [target]: s.state }));
    s.start();
  }

  async function disconnect(target: string) {
    const s = sessions()[target];
    if (!s) return;
    offStates.get(target)?.(); // close() の closed を一覧に書き戻させない
    offStates.delete(target);
    setSessions(({ [target]: _, ...rest }) => rest);
    setStates(({ [target]: _, ...rest }) => rest);
    await s.close();
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
          {(t) => {
            const st = () => states()[t];
            const kind = () => st()?.kind;
            return (
              <li data-state={kind() ?? "idle"}>
                <button
                  disabled={
                    kind() === "connecting" ||
                    kind() === "connected" ||
                    kind() === "reconnecting"
                  }
                  onClick={() => connect(t)}
                >
                  {label(t)}
                </button>{" "}
                <span role="status">{describeState(st(), now())}</span>
                <Show
                  when={
                    (kind() === "reconnecting" || kind() === "connecting") &&
                    (st() as { retryAt?: number }).retryAt !== undefined
                  }
                >
                  {" "}
                  <button onClick={() => sessions()[t]?.retryNow()}>
                    今すぐつなぐ
                  </button>
                </Show>
                <Show when={kind() === "failed" || kind() === "superseded"}>
                  {" "}
                  <button onClick={() => connect(t)}>つなぎ直す</button>
                </Show>
                <Show when={st() && kind() !== "closed"}>
                  {" "}
                  <button onClick={() => void disconnect(t)}>切る</button>
                </Show>
                <Show when={st()?.kind === "failed"}>
                  <pre role="alert">
                    {(st() as { message: string }).message}
                  </pre>
                </Show>
              </li>
            );
          }}
        </For>
      </ul>
      <AskpassDialog ask={asks()[0]} onReply={reply} />
    </main>
  );
}
