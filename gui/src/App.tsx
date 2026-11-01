// 接続の画面（C3〜C7）。起動時は、前回ワークスペースがあったマシンにだけ自動でつなぐ（C5）。
// 左の一覧の一番下の「ホストを開く」で窓を出し、つなぐ先（このマシンと ~/.ssh/config のホスト）を選んで
// shj につなぐ（connect/HostPicker.tsx）。ssh の質問（パスワードなど）と shj を入れるかの確認は窓に出す。
// 切れたら Session が自動でつなぎ直し、左の一覧の見出しに接続の状態を出す（丸・1 行・ボタン）。
// つながらない・別の GUI に引き継がれた（C7）ときは右下に知らせ、ボタンでつなぎ直せるようにする（connect/Notices.tsx）。
// 左にはつないだマシンごとの見出しとワークスペースの帯を出し、帯を押すとフォーカスを移す（D1）。
// フォーカスしているワークスペースの帯の下にはツリーを出す。開閉とスクロールの位置はワークスペースごとに覚え、
// フォーカスを外して戻しても残す（D2）。マシンごとに色を決めて覚え、見出しと帯の左端の線に使う（D4）。
// 右上にはフォーカスしているマシンの shj が送る画面のファイルの欄を描く（E1）。欄の大きさは、つないだ
// どのマシンにも送る（どのマシンにフォーカスを移しても、同じ大きさの欄に描くため）。
// 欄で受けたキー・IME・マウス・貼り付けは、フォーカスしているマシンの shj に送る。shj でコピーしたら
// OS のクリップボードに入れる（E2）。
// 帯を押してフォーカスを移すと、右下のターミナルもそのワークスペースのシェルに切り替える（F2）。
// Ctrl+Shift+` でファイルの欄とターミナルのカードを入れ替える（IDE モードとターミナルモード。F3。mode.ts）。
// ターミナルモードでは、マシンのターミナルを数に応じて自動で並べる。Ctrl+Shift+T で増やし、`exit` で閉じて
// 並び直し、Ctrl+Shift+矢印で隣へ移る（F4。terminal/group.ts・terminal/grid.ts）。
// 新しい版があれば右下に知らせ、押されたら入れ替えて起動し直す（C8。update/UpdateBar.tsx）。
// 窓と知らせは、どれもモック Connect.dc.html のカードの決まりで描く（connect/card.css）。
import {
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
  onMount,
  Show,
  type JSX,
  untrack,
} from "solid-js";

import { AskpassDialog } from "./AskpassDialog.tsx";
import { HostPicker } from "./connect/HostPicker.tsx";
import { InstallCard } from "./connect/InstallCard.tsx";
import { Notices, type Notice } from "./connect/Notices.tsx";
import { describeClient, statusLine } from "./connect/status.ts";
import { UpdateBar } from "./update/UpdateBar.tsx";
import { BlobStore } from "./files/blob.ts";
import { OsClipboard } from "./files/clipboard.ts";
import {
  FilesView,
  type FilesSize,
  type SendInput,
} from "./files/FilesView.tsx";
import type { Screen, Theme, Workspace, WorkspaceId } from "./rpc/protocol.ts";
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
import { directionKey, isNewTermKey } from "./terminal/grid.ts";
import { TermGroup } from "./terminal/group.ts";
import { TerminalGrid } from "./terminal/TerminalGrid.tsx";
import { isModeKey, otherMode, swapWithMotion, type Mode } from "./mode.ts";

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
  // マシンごとのターミナル（F1・F4）。フォーカスしているマシン（無ければ最初につながったマシン）のものを出す。
  // ワークスペースはシェルの cwd から決まるので、フォーカスが無くても出す。並びが変わったら termVersion を進める
  const termGroups = new Map<string, TermGroup>();
  const [termVersion, setTermVersion] = createSignal(0);
  const termTarget = createMemo(
    () =>
      focus()?.target ??
      [LOCAL, ...hosts()].find((t) => states()[t]?.kind === "connected"),
  );
  const termGroup = createMemo(() => {
    const t = termTarget();
    return t !== undefined && sessions()[t] ? termGroups.get(t) : undefined;
  });
  // 並びと今使っている欄（TermGroup は signal ではないので、termVersion で描き直す）
  const termList = () => {
    void termVersion();
    return termGroup()?.list ?? [];
  };
  const termActive = () => {
    void termVersion();
    return termGroup()?.active;
  };
  // 出すマシンが変わったら、前のマシンのものを外し、出すマシンのシェルを拾う
  createEffect<TermGroup | undefined>((prev) => {
    const g = termGroup();
    if (g !== prev) {
      prev?.hide();
      g?.show();
    }
    return g;
  });
  // マシンごとのツリー（D2）。中身は WorkspaceTrees が持ち、変わったら treeVersion を進めて描き直す
  const trees = new Map<string, WorkspaceTrees>();
  const [treeVersion, setTreeVersion] = createSignal(0);
  // マシンごとの画像の実体（E6）。blob.get で取ったものを覚えておく
  const blobStores = new Map<string, BlobStore>();
  // マシンごとの画面とテーマ（E1）。Session が持つものを写す
  const [screens, setScreens] = createSignal<Record<string, Screen | null>>({});
  const [themes, setThemes] = createSignal<Record<string, Theme | null>>({});
  // ファイルの欄の大きさ。新しくつなぐマシンにも送る
  let filesSize: FilesSize | undefined;
  const clipboard = new OsClipboard(
    () => navigator.clipboard,
    (e) => console.warn("clipboard:", e),
  );
  let nav: HTMLElement | undefined;
  // IDE モード（上がファイルの欄）かターミナルモード（上がターミナル）か（F3）
  const [mode, setMode] = createSignal<Mode>("ide");
  let mainEl: HTMLDivElement | undefined;
  const [asks, setAsks] = createSignal<AskpassRequest[]>([]);
  // ホストを開く窓を出しているか
  const [picking, setPicking] = createSignal(false);
  // 右下の知らせのうち、閉じたもの（状態が変われば、また出す）
  const [dismissed, setDismissed] = createSignal<ReadonlySet<SessionState>>(
    new Set(),
  );
  // 「あと何秒」を動かす
  const [now, setNow] = createSignal(Date.now());
  const tick = setInterval(() => setNow(Date.now()), 1000);

  // await の後は Owner が無いので、onCleanup は先に登録する
  const offs: (() => void)[] = [];
  // GUI が受けるキーなので、欄やターミナルに届く前（capture）に取って、shj にもシェルにも送らない
  const onGuiKey = (e: KeyboardEvent) => {
    if (isModeKey(e)) {
      e.preventDefault();
      e.stopPropagation();
      if (!e.repeat) toggleMode();
      return;
    }
    if (isNewTermKey(e)) {
      e.preventDefault();
      e.stopPropagation();
      if (!e.repeat) newTerminal();
      return;
    }
    // 隣へ移るのは、ターミナルを並べていて、キーがターミナルに向いているときだけ（ファイルの欄の Ctrl+Shift+矢印は shj に送る）
    const dir = directionKey(e);
    const card = mainEl?.querySelector(":scope > .terminal");
    if (dir && mode() === "terminal" && card?.contains(e.target as Node)) {
      e.preventDefault();
      e.stopPropagation();
      if (termGroup()?.move(dir)) focusTerminal();
    }
  };
  window.addEventListener("keydown", onGuiKey, true);
  onCleanup(() => {
    window.removeEventListener("keydown", onGuiKey, true);
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
    const group = new TermGroup(
      s,
      () => setTermVersion((v) => v + 1),
      (m) => console.warn(`[${target}] ${m}`),
    );
    termGroups.set(target, group);
    blobStores.set(
      target,
      new BlobStore((params) => s.request("blob.get", params)),
    );
    const offTerm = s.onTerm((e) => group.handle(e));
    const off = s.onState((st) => {
      // つなぎ直したら、ターミナルの溜めた出力を続きから取り直す（右下に出す前なら何もしない）
      if (st.kind === "connected") group.reconnected();
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
    const showScreen = () => setScreens((m) => ({ ...m, [target]: s.screen }));
    const offScreen = s.onScreen(showScreen);
    const offUpdate = s.onUpdate(showScreen);
    const offTheme = s.onTheme((th) =>
      setThemes((m) => ({ ...m, [target]: th })),
    );
    const offClipboard = s.onClipboard((text) => void clipboard.write(text));
    offStates.set(target, () => {
      offTerm();
      off();
      offTree();
      offGit();
      offWs();
      offFocus();
      offScreen();
      offUpdate();
      offTheme();
      offClipboard();
    });
    if (filesSize) s.resize(filesSize);
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
    termGroups.delete(target); // シェルは止めない（次につないだら term.list で見つけてつなぐ）
    blobStores.get(target)?.clear();
    blobStores.delete(target);
    setSessions(({ [target]: _, ...rest }) => rest);
    setStates(({ [target]: _, ...rest }) => rest);
    setWorkspaces(({ [target]: _, ...rest }) => rest);
    setScreens(({ [target]: _, ...rest }) => rest);
    setThemes(({ [target]: _, ...rest }) => rest);
    setFocus((f) => keepFocus(f, workspaces(), shjFocuses()));
    await s.close();
  }

  /** ファイルの欄の大きさが変わった。つないでいるどのマシンにも送る。 */
  function resizeFiles(size: FilesSize) {
    filesSize = size;
    for (const s of Object.values(sessions())) s.resize(size);
  }

  /** ファイルの欄で受けた入力を、フォーカスしているマシンの shj に送る。 */
  const sendInput: SendInput = (method, params) => {
    const f = focus();
    if (f) sessions()[f.target]?.input(method, params);
  };

  /** ワークスペースのルート（分からなければ undefined）。 */
  function rootOf(target: string, id: WorkspaceId): string | undefined {
    return workspaces()[target]?.find((w) => w.id === id)?.root;
  }

  /** 帯を押した。shj に workspace.focus を送り、受け付けられたら GUI のフォーカスも移す。 */
  async function focusWorkspace(target: string, id: WorkspaceId) {
    const s = sessions()[target];
    if (!s) return;
    const seq = ++focusSeq; // 続けて押したら、最後に押したものだけを使う
    setFocusError("");
    try {
      const termId = await s.focusWorkspace(id);
      if (seq !== focusSeq) return;
      // 右下のターミナルも、そのワークスペースで最後に使ったシェルにする（無ければルートで作る）（F2）
      const root = rootOf(target, id);
      if (root !== undefined) termGroups.get(target)?.switchTo(termId, root);
      setFocus({ target, id });
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

  /** IDE モードとターミナルモードを入れ替え、上に来た大きいカードにキーを向ける（F3）。 */
  function toggleMode() {
    const next = otherMode(mode());
    const files = mainEl?.querySelector<HTMLElement>(":scope > .files");
    const term = mainEl?.querySelector<HTMLElement>(":scope > .terminal");
    swapWithMotion([files, term], () => setMode(next));
    // ファイルの欄は入力用の textarea、ターミナルは今使っている欄の xterm.js の textarea がキーを受ける
    if (next === "terminal") focusTerminal();
    else
      files
        ?.querySelector<HTMLElement>(".files-ime")
        ?.focus({ preventScroll: true });
  }

  /** 今使っているターミナルの欄にキーを向ける。 */
  function focusTerminal() {
    // 足した欄の xterm.js が開くのを待つ
    queueMicrotask(() =>
      mainEl
        ?.querySelector<HTMLElement>(
          ":scope > .terminal .terminal-cell[data-active] textarea",
        )
        ?.focus({ preventScroll: true }),
    );
  }

  /** 新しいターミナルを作って並べ、キーを向ける（Ctrl+Shift+T。F4）。フォーカスしているワークスペースのルートで開く。 */
  function newTerminal() {
    const g = termGroup();
    if (!g) return;
    const f = focus();
    const root = f ? rootOf(f.target, f.id) : undefined;
    // 無ければ shj に任せる（フォーカスしているワークスペースのルート、それも無ければ ~）
    if (g.add(root)) focusTerminal();
  }

  /** 見出しのそばのボタン。待ち時間中なら今すぐ、使い切った・引き継がれたあとならつなぎ直す。 */
  function retry(target: string) {
    const s = sessions()[target];
    if (!s) return;
    const st = s.state;
    if (st.kind === "failed" || st.kind === "superseded") connect(target);
    else if (
      (st.kind === "connecting" || st.kind === "reconnecting") &&
      st.retryAt !== undefined
    )
      s.retryNow();
  }

  // shj を入れるかを聞くマシン（一度に 1 台。答えたら次のマシン）
  const installing = createMemo(() =>
    [LOCAL, ...hosts(), ...Object.keys(states())].find(
      (t) => states()[t]?.kind === "needs-install",
    ),
  );

  // 知らせのオブジェクトは、元の状態（SessionState・focusError の文）が同じなら使い回す。
  // 作り直すと Notices が描き直し、ほかのマシンの状態が変わるだけで読み上げ直し、ボタンのフォーカスも消えるため
  const noticeOf = new WeakMap<SessionState, Notice | null>();
  let focusNotice: { err: string; notice: Notice } | undefined;

  /** 右下に積む知らせ。つながらない・別の GUI に移った・ワークスペースに移れなかった。 */
  const notices = createMemo<Notice[]>(() => {
    const out: Notice[] = [];
    const err = focusError();
    if (err) {
      if (focusNotice?.err !== err) {
        focusNotice = {
          err,
          notice: {
            title: "ワークスペースに移れませんでした",
            error: err,
            primary: { label: "閉じる", run: () => setFocusError("") },
          },
        };
      }
      out.push(focusNotice.notice);
    }
    for (const [t, st] of Object.entries(states())) {
      if (dismissed().has(st)) continue;
      if (!noticeOf.has(st)) noticeOf.set(st, makeNotice(t, st));
      const n = noticeOf.get(st);
      if (n) out.push(n);
    }
    return out;
  });

  /** マシン 1 台の状態の知らせ。知らせることが無ければ null。 */
  function makeNotice(t: string, st: SessionState): Notice | null {
    const label = machineLabel(t);
    const close = {
      label: "閉じる",
      run: () => setDismissed((d) => new Set(d).add(st)),
    };
    const cut = { label: "切る", run: () => void disconnect(t) };
    if (st.kind === "failed") {
      return {
        title: `${label} につながりません`,
        error: st.message,
        secondary: [close, cut],
        primary: { label: "つなぎ直す", run: () => connect(t) },
      };
    }
    if (st.kind === "superseded") {
      const who = st.by ? describeClient(st.by) : "";
      return {
        title: who
          ? `${label} は別の GUI に移りました（${who}）`
          : `${label} は別の GUI に移りました`,
        text:
          `${label} の作業は、あとからつないだ別の GUI に移りました。` +
          "取り合いにならないよう、自動ではつなぎ直しません。" +
          "「つなぎ直す」を押すとこちらに戻し、あちらの GUI が切れます。",
        secondary: [close, cut],
        primary: { label: "つなぎ直す", run: () => connect(t) },
      };
    }
    return null;
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
        statusOf={(t) => statusLine(states()[t], now())}
        onRetry={retry}
        onDisconnect={(t) => void disconnect(t)}
        onOpenHost={() => setPicking(true)}
        ref={(el) => {
          nav = el;
          el.addEventListener("scroll", rememberScroll, { passive: true });
        }}
      />
      <div class="main" data-mode={mode()} ref={mainEl}>
        <FilesView
          screen={focus() ? (screens()[focus()!.target] ?? null) : null}
          theme={focus() ? (themes()[focus()!.target] ?? null) : null}
          onResize={resizeFiles}
          onInput={sendInput}
          clipboard={clipboard}
          blobs={
            focus() && sessions()[focus()!.target]
              ? blobStores.get(focus()!.target)
              : undefined
          }
        />
        <TerminalGrid
          links={termList()}
          active={termActive()}
          showAll={mode() === "terminal"}
          accent={colors()[termTarget() ?? ""]}
          onActivate={(i) => termGroup()?.setActive(i)}
        />
      </div>
      <Show when={installing()}>
        {(t) => (
          <InstallCard
            target={machineLabel(t())}
            state={
              states()[t()] as Extract<SessionState, { kind: "needs-install" }>
            }
            onYes={() => void sessions()[t()]?.install()}
            onNo={() => sessions()[t()]?.declineInstall()}
          />
        )}
      </Show>
      <Show when={picking()}>
        <HostPicker
          targets={[LOCAL, ...hosts()]}
          stateOf={(t) => states()[t]}
          onConnect={connect}
          onClose={() => setPicking(false)}
        />
      </Show>
      <AskpassDialog ask={asks()[0]} onReply={reply} />
      <Notices notices={notices()}>
        <UpdateBar />
      </Notices>
    </main>
  );
}
