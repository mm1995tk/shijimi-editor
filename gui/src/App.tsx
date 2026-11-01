// 接続の画面（C3〜C7）。起動時は、前回ワークスペースがあったマシンにだけ自動でつなぐ（C5）。
// つなぐ先（このマシンと ~/.ssh/config のホスト）を選んで shj につなぎ、
// ssh の質問（パスワードなど）を窓に出す。切れたら Session が自動でつなぎ直し、
// つなぐ先の一覧に接続の状態を出す。別の GUI に引き継がれたら、そう出してボタンでつなぎ直せるようにする（C7）。
// 左にはつないだマシンごとの見出しとワークスペースの帯を出し、帯を押すとフォーカスを移す（D1）。
// フォーカスしているワークスペースの帯の下にはツリーを出す。開閉とスクロールの位置はワークスペースごとに覚え、
// フォーカスを外して戻しても残す（D2）。マシンごとに色を決めて覚え、見出しと帯の左端の線に使う（D4）。
// 編集の画面は E 以降で作る。
import {
  createEffect,
  createMemo,
  createSignal,
  For,
  onCleanup,
  onMount,
  Show,
  type JSX,
  untrack,
} from "solid-js";

import { AskpassDialog } from "./AskpassDialog.tsx";
import type { Workspace, WorkspaceId } from "./rpc/protocol.ts";
import {
  adoptFocus,
  buildSidebar,
  keepFocus,
  machineLabel,
  type WorkspaceFocus,
} from "./sidebar/model.ts";
import { machineColors } from "./sidebar/colors.ts";
import { Sidebar } from "./sidebar/Sidebar.tsx";
import "./app.css";
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
import { rows as treeRows } from "./tree/model.ts";
import { WorkspaceTrees } from "./tree/trees.ts";
import { TermLink } from "./terminal/link.ts";
import { TerminalView } from "./terminal/TerminalView.tsx";

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
    case "needs-install":
      return "shj がありません";
    case "installing":
      return "shj を入れています…";
    case "failed":
      return "つながりません";
    case "superseded": {
      const who = s.by ? describeClient(s.by) : "";
      return who ? `別の GUI に移りました（${who}）` : "別の GUI に移りました";
    }
    case "closed":
      return "";
  }
}

/** 引き継いだ GUI を短く言う（「名前 版、HH:MM から」）。読めないところは省き、何も無ければ ""。 */
export function describeClient(
  by: NonNullable<Extract<SessionState, { kind: "superseded" }>["by"]>,
): string {
  const name = [by.client?.name, by.client?.version]
    .filter((x) => typeof x === "string" && x !== "")
    .join(" ");
  const at = new Date(by.connectedAt);
  const time = Number.isNaN(at.getTime())
    ? ""
    : `${at.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })} から`;
  return [name, time].filter((x) => x !== "").join("、");
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
  const [workspaces, setWorkspaces] = createSignal<Record<string, Workspace[]>>(
    {},
  );
  // GUI のフォーカスは 1 つ（どのマシンの、どのワークスペースか）
  const [focus, setFocus] = createSignal<WorkspaceFocus | null>(null);
  const [focusError, setFocusError] = createSignal("");
  let focusSeq = 0;
  const offStates = new Map<string, () => void>();
  // マシンごとのターミナル（F1）。右下には、フォーカスしているマシン（無ければ最初につながったマシン）の
  // ものを 1 つ出す。ワークスペースはシェルの cwd から決まるので、フォーカスが無くても出す
  const termLinks = new Map<string, TermLink>();
  const termTarget = createMemo(
    () =>
      focus()?.target ??
      [LOCAL, ...hosts()].find((t) => states()[t]?.kind === "connected"),
  );
  const termLink = createMemo(() => {
    const t = termTarget();
    return t !== undefined && sessions()[t] ? termLinks.get(t) : undefined;
  });
  // マシンごとのツリー（D2）。中身は WorkspaceTrees が持ち、変わったら treeVersion を進めて描き直す
  const trees = new Map<string, WorkspaceTrees>();
  const [treeVersion, setTreeVersion] = createSignal(0);
  let nav: HTMLElement | undefined;
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

  /** マシンごとの shj のフォーカス（GUI のフォーカスが外れたときの代わりの候補）。 */
  function shjFocuses(): Record<string, WorkspaceId | null> {
    return Object.fromEntries(
      Object.entries(sessions()).map(([t, s]) => [t, s.focus]),
    );
  }

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
    const tr = new WorkspaceTrees({
      request: (method, params) => s.request(method, params),
      onChange: () => setTreeVersion((v) => v + 1),
      onError: (m) => console.warn(`[${target}] ${m}`),
    });
    trees.set(target, tr);
    const link = new TermLink(s, (m) => console.warn(`[${target}] ${m}`));
    termLinks.set(target, link);
    const offTerm = s.onTerm((e) => link.handle(e));
    const off = s.onState((st) => {
      // つなぎ直したら、ターミナルの溜めた出力を続きから取り直す（右下に出す前なら何もしない）
      if (st.kind === "connected") link.reconnected();
      setStates((m) => ({ ...m, [target]: st }));
      // つなぎ直したら、今のワークスペースのツリーと git の印を取り直す
      if (st.kind === "connected") tr.reconnected();
    });
    const offTree = s.onTreeChanged((n) => tr.handleTreeChanged(n));
    const offGit = s.onGitChanged((n) => tr.handleGitChanged(n));
    const offWs = s.onWorkspaces((ws) => {
      tr.retain(ws.map((w) => w.id));
      recordWorkspaces(store, target, ws.length);
      setWorkspaces((m) => ({ ...m, [target]: ws }));
      setFocus((f) => keepFocus(f, workspaces(), shjFocuses()));
    });
    const offFocus = s.onFocus((id) =>
      setFocus((f) =>
        keepFocus(adoptFocus(f, target, id), workspaces(), shjFocuses()),
      ),
    );
    offStates.set(target, () => {
      offTerm();
      off();
      offTree();
      offGit();
      offWs();
      offFocus();
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
    trees.delete(target);
    termLinks.delete(target); // シェルは止めない（次につないだら term.list で見つけてつなぐ）
    setSessions(({ [target]: _, ...rest }) => rest);
    setStates(({ [target]: _, ...rest }) => rest);
    setWorkspaces(({ [target]: _, ...rest }) => rest);
    setFocus((f) => keepFocus(f, workspaces(), shjFocuses()));
    await s.close();
  }

  /** 帯を押した。shj に workspace.focus を送り、受け付けられたら GUI のフォーカスも移す。 */
  async function focusWorkspace(target: string, id: WorkspaceId) {
    const s = sessions()[target];
    if (!s) return;
    const seq = ++focusSeq; // 続けて押したら、最後に押したものだけを使う
    setFocusError("");
    try {
      await s.focusWorkspace(id);
      if (seq === focusSeq) setFocus({ target, id });
    } catch (e) {
      if (seq !== focusSeq) return;
      setFocusError(
        `${machineLabel(target)} のワークスペースに移れませんでした：${
          e instanceof Error ? e.message : String(e)
        }`,
      );
    }
  }

  // 初めて出たマシンには色を割り当てて保存する（つなぐマシンが変わったときだけ計算し直す）
  const colors = createMemo(() =>
    machineColors(store, Object.keys(sessions())),
  );
  const groups = createMemo(() =>
    buildSidebar(
      Object.keys(sessions()),
      states(),
      workspaces(),
      focus(),
      colors(),
    ),
  );

  // フォーカスしているワークスペースのツリーだけを動かし（監視と取り直し）、
  // 前にそのワークスペースで覚えたスクロールの位置に戻す（D2）
  createEffect(() => {
    const f = focus();
    Object.keys(sessions()); // つないだマシンが増えたら、その WorkspaceTrees にも伝える
    untrack(() => {
      let restore: number | undefined;
      for (const [t, tr] of trees) {
        const id = f?.target === t ? f.id : null;
        if (id !== null && id !== tr.activeId) restore = tr.get(id)?.scrollTop;
        tr.activate(id);
      }
      // 描き直し（ツリーの入れ替え）のあとで戻す。初めてのワークスペースは今の位置のまま
      if (restore !== undefined) {
        const top = restore;
        requestAnimationFrame(() => {
          if (nav) nav.scrollTop = top;
        });
      }
    });
  });

  function rememberScroll() {
    const f = focus();
    if (f && nav) trees.get(f.target)?.setScroll(f.id, nav.scrollTop);
  }

  function treeOf(target: string, id: WorkspaceId) {
    treeVersion();
    const s = trees.get(target)?.get(id);
    return s && s.listings[""] ? treeRows(s) : undefined;
  }

  function reply(requestId: number, value: string | null) {
    setAsks((a) => a.filter((x) => x.requestId !== requestId));
    void api.askpassReply(requestId, value);
  }

  return (
    <main aria-label="shijimi-editor" class="app">
      <Sidebar
        groups={groups()}
        onFocus={(t, id) => void focusWorkspace(t, id)}
        treeOf={treeOf}
        onToggle={(t, id, p) => trees.get(t)?.toggle(id, p)}
        ref={(el) => {
          nav = el;
          el.addEventListener("scroll", rememberScroll, { passive: true });
        }}
      />
      <section class="connections" aria-label="つなぐ先">
        <h1>つなぐ先</h1>
        <Show when={focusError()}>
          <p role="alert">{focusError()}</p>
        </Show>
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
                      kind() === "reconnecting" ||
                      kind() === "needs-install" ||
                      kind() === "installing"
                    }
                    onClick={() => connect(t)}
                  >
                    {machineLabel(t)}
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
                  <Show when={st()?.kind === "needs-install"}>
                    <InstallPrompt
                      target={machineLabel(t)}
                      state={
                        st() as Extract<SessionState, { kind: "needs-install" }>
                      }
                      onYes={() => void sessions()[t]?.install()}
                      onNo={() => sessions()[t]?.declineInstall()}
                    />
                  </Show>
                  <Show when={kind() === "superseded"}>
                    <p role="alert">
                      {machineLabel(t)} の作業は、あとからつないだ別の GUI
                      に移りました。
                      取り合いにならないよう、自動ではつなぎ直しません。
                      「つなぎ直す」を押すとこちらに戻し、あちらの GUI
                      が切れます。
                    </p>
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
      </section>
      <TerminalView link={termLink()} />
      <AskpassDialog ask={asks()[0]} onReply={reply} />
    </main>
  );
}

/**
 * shj が無いマシンに入れてよいかを聞く。何を・どこから・どこへ入れるかを見せ、
 * 「入れる」と押されるまで何もしない。
 */
function InstallPrompt(props: {
  target: string;
  state: Extract<SessionState, { kind: "needs-install" }>;
  onYes: () => void;
  onNo: () => void;
}): JSX.Element {
  return (
    <div role="group" aria-label="shj を入れるか">
      <p>
        {props.target} に shj がありません。GUI と同じ版（
        {props.state.plan.version}）の shj を GitHub Releases（
        {props.state.plan.release_url}）から取り、{props.target} の{" "}
        <code>{props.state.plan.path}</code> に入れてよいですか？
        取った実行ファイルは SHA-256
        を確かめてから使い、合わなければ入れません。PATH の shj には触りません。
      </p>
      <button onClick={props.onYes}>入れる</button>{" "}
      <button onClick={props.onNo}>入れない</button>
      <Show when={props.state.error}>
        <pre role="alert">{props.state.error}</pre>
      </Show>
    </div>
  );
}
