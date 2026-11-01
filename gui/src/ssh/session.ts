// 切れたら自動でつなぎ直す接続（C4）。connectShj の上に、次の 3 つを足す。
//  - session.ping を一定間隔で送り、ssh が黙って切れたことに気づく
//  - 切れたら間隔を空けて何度かつなぎ直す。session.hello で resume を送り、同じ serverId なら
//    screen.get で全部取り直す。違えばワークスペースの一覧（hello.workspaces）から取り直す
//  - 状態（State）を外に出す。一覧の表示はこれを読む
// Solid などの画面の仕組みには依存しない。

import type { Client } from "../rpc/client.ts";
import type {
  HelloResult,
  Notifications,
  Screen,
  ScreenUpdate,
} from "../rpc/protocol.ts";
import type { InstallPlan, ShellApi } from "./api.ts";
import {
  connectShj,
  newId,
  ShjMissingError,
  type Connection,
} from "./connect.ts";

/** 接続の状態。一覧の表示はこれを読む。 */
export type SessionState =
  /** まだ一度もつながっていない。2 回目以降の試みでは max と reason、待ち時間中は retryAt が付く */
  | {
      kind: "connecting";
      attempt: number;
      max: number;
      retryAt?: number;
      reason?: string;
    }
  | { kind: "connected"; hello: HelloResult }
  /** 切れて、つなぎ直す順番を待っている（または試している） */
  | {
      kind: "reconnecting";
      /** 何回目の試みか（1 始まり） */
      attempt: number;
      max: number;
      /** 次の試みの時刻（ms）。試している最中は undefined */
      retryAt?: number;
      reason: string;
    }
  /**
   * リモートに shj が無い。ユーザーに聞くまで何も入れない。install() で入れ、declineInstall() で断る。
   * error は直前の入れ方が失敗した理由。
   */
  | { kind: "needs-install"; plan: InstallPlan; error?: string }
  /** shj を取って、確かめて、リモートに送っている */
  | { kind: "installing"; plan: InstallPlan }
  /** 回数を使い切った。ボタンで retry() できる */
  | { kind: "failed"; message: string }
  /** 別の GUI に引き継がれた。自動ではつなぎ直さない（取り合いになるため。表示は C7） */
  | { kind: "superseded" }
  | { kind: "closed" };

/** Session の調整値。どれも省くと下の既定値。 */
export interface SessionOptions {
  /** つなぎ直しの間隔（ms）。n 回目の失敗の後は delays[n-1]、尽きたら最後の値。 */
  delays?: number[];
  /** 諦めるまでのつなぎ直しの回数。 */
  maxAttempts?: number;
  /** session.ping を送る間隔（ms） */
  pingIntervalMs?: number;
  /** ping の返事を待つ時間。これを過ぎたら切れたとみなす */
  pingTimeoutMs?: number;
  /** 読めなかった行などの報告先（ログ用） */
  onDiscard?: (reason: string, line: string) => void;
}

/** 1, 2, 4, 8, 16, 30 秒…（最後の値を繰り返す）。10 回で待ちは合わせて 151 秒ほど（ssh の接続待ちは別）。 */
export const DEFAULT_DELAYS = [1000, 2000, 4000, 8000, 16000, 30000];
/** 諦めるまでの試みの回数 */
export const DEFAULT_MAX_ATTEMPTS = 10;
/** 設計書 3 章：GUI は 15 秒ごとに session.ping を送る */
export const PING_INTERVAL_MS = 15000;
/** ping の返事を待つ時間（ms） */
export const PING_TIMEOUT_MS = 10000;

/**
 * 1 つのつなぎ先への接続。切れたら自動でつなぎ直し、state で今の様子を伝える。
 * start() で始め、close() で閉じる。
 */
export class Session {
  readonly target: string;
  private readonly api: ShellApi;
  private readonly opts: Required<Omit<SessionOptions, "onDiscard">> &
    Pick<SessionOptions, "onDiscard">;
  private _state: SessionState = { kind: "connecting", attempt: 0, max: 0 };
  private conn?: Connection;
  /** 最後につながった shj の serverId と、GUI が持つ画面の版 */
  private serverId?: string;
  private rev = 0;
  /** screen.get の返事待ち。この間の screen.update は捨てる */
  private fetching = false;
  private _screen: Screen | null = null;
  private _workspaces: HelloResult["workspaces"] = [];
  private wake?: () => void;
  /** 入れている最中の id（「切る」で askpass の質問を取り消すのに使う） */
  private installId?: string;
  private generation = 0;
  private readonly stateHandlers = new Set<(s: SessionState) => void>();
  private readonly screenHandlers = new Set<(s: Screen) => void>();
  private readonly updateHandlers = new Set<(u: ScreenUpdate) => void>();
  private readonly workspaceHandlers = new Set<
    (w: HelloResult["workspaces"]) => void
  >();

  constructor(api: ShellApi, target: string, opts: SessionOptions = {}) {
    this.api = api;
    this.target = target;
    this.opts = {
      delays: opts.delays ?? DEFAULT_DELAYS,
      maxAttempts: opts.maxAttempts ?? DEFAULT_MAX_ATTEMPTS,
      pingIntervalMs: opts.pingIntervalMs ?? PING_INTERVAL_MS,
      pingTimeoutMs: opts.pingTimeoutMs ?? PING_TIMEOUT_MS,
      onDiscard: opts.onDiscard,
    };
  }

  get state(): SessionState {
    return this._state;
  }
  /** 最後に取った画面（つなぎ直しのたびに取り直す）。 */
  get screen(): Screen | null {
    return this._screen;
  }
  get workspaces(): HelloResult["workspaces"] {
    return this._workspaces;
  }
  get connection(): Connection | undefined {
    return this.conn;
  }

  onState(h: (s: SessionState) => void): () => void {
    this.stateHandlers.add(h);
    return () => this.stateHandlers.delete(h);
  }
  /** ワークスペースの一覧が変わったとき（つながったとき、workspace.changed）。 */
  onWorkspaces(h: (w: HelloResult["workspaces"]) => void): () => void {
    this.workspaceHandlers.add(h);
    return () => this.workspaceHandlers.delete(h);
  }
  /** 全部の画面を取り直したとき（つなぎ直し、版の食い違い）。 */
  onScreen(h: (s: Screen) => void): () => void {
    this.screenHandlers.add(h);
    return () => this.screenHandlers.delete(h);
  }
  /** 差分の更新（版が続いているものだけ）。 */
  onUpdate(h: (u: ScreenUpdate) => void): () => void {
    this.updateHandlers.add(h);
    return () => this.updateHandlers.delete(h);
  }

  /** つなぎ始める。最初の接続で失敗したときも、つなぎ直しと同じ回数まで試す。 */
  start(): void {
    void this.run(0, "");
  }

  /** 諦めた後に、回数を数え直してつなぐ。 */
  retry(): void {
    if (this._state.kind !== "failed" && this._state.kind !== "superseded")
      return;
    void this.run(0, "");
  }

  /** 待たずに今すぐつなぎ直す（つなぎ直しの待ち時間のとき）。 */
  retryNow(): void {
    this.wake?.();
  }

  /**
   * shj が無いリモートに、ユーザーが「入れる」と答えたあとで呼ぶ。GUI と同じ版の shj を取り、
   * SHA-256 を確かめて入れ、つなぎ直す。失敗したら needs-install（error 付き）に戻る。
   */
  async install(): Promise<void> {
    const s = this._state;
    if (s.kind !== "needs-install") return;
    const gen = ++this.generation;
    const plan = s.plan;
    this.setState({ kind: "installing", plan });
    const id = newId();
    this.installId = id;
    try {
      await this.api.installRemote(id, this.target);
    } catch (e) {
      if (this.installId === id) this.installId = undefined;
      if (gen !== this.generation) return;
      this.setState({
        kind: "needs-install",
        plan,
        error: e instanceof Error ? e.message : String(e),
      });
      return;
    }
    if (this.installId === id) this.installId = undefined;
    if (gen !== this.generation) return;
    await this.run(0, "");
  }

  /** 入れない。何も入れず、「つながりません」にする（つなぎ直すと、もう一度聞く）。 */
  declineInstall(): void {
    if (this._state.kind !== "needs-install") return;
    this.generation++;
    this.setState({
      kind: "failed",
      message: `shj が ${this.target} にありません（入れませんでした）`,
    });
  }

  private async askInstall(gen: number): Promise<void> {
    try {
      const plan = await this.api.installPlan();
      if (gen === this.generation)
        this.setState({ kind: "needs-install", plan });
    } catch (e) {
      if (gen !== this.generation) return;
      this.setState({
        kind: "failed",
        message: e instanceof Error ? e.message : String(e),
      });
    }
  }

  /** 閉じる（session.bye。shj とシェルは止まらない）。 */
  async close(): Promise<void> {
    this.generation++;
    this.wake?.();
    const c = this.conn;
    this.conn = undefined;
    this.setState({ kind: "closed" });
    // 入れている最中なら、待っている askpass の質問を取り消す（窓が残って、答えると入れ続けるのを防ぐ）
    const installId = this.installId;
    this.installId = undefined;
    if (installId) void this.api.installCancel(installId).catch(() => {});
    await c?.close();
  }

  private setState(s: SessionState) {
    this._state = s;
    for (const h of [...this.stateHandlers]) h(s);
  }

  private sleep(ms: number): Promise<void> {
    return new Promise((resolve) => {
      const done = () => {
        clearTimeout(t);
        this.wake = undefined;
        resolve();
      };
      const t = setTimeout(done, ms);
      this.wake = done;
    });
  }

  /** まだ一度もつながっていなければ connecting、一度つながっていれば reconnecting。 */
  private waiting(
    first: boolean,
    attempt: number,
    reason: string,
    retryAt?: number,
  ): SessionState {
    const max = this.opts.maxAttempts;
    if (first) {
      return attempt === 1
        ? { kind: "connecting", attempt, max }
        : { kind: "connecting", attempt, max, retryAt, reason };
    }
    return { kind: "reconnecting", attempt, max, retryAt, reason };
  }

  /**
   * 試みを繰り返す。startAttempt は数え始めの回数。reason は直前に切れた理由。
   * 古い run は generation が合わなくなって手を引く。
   */
  private async run(startAttempt: number, reason: string): Promise<void> {
    const gen = ++this.generation;
    const first = this.serverId === undefined;
    let lastError = reason;
    for (let attempt = startAttempt + 1; ; attempt++) {
      if (attempt > this.opts.maxAttempts) {
        this.setState({ kind: "failed", message: lastError });
        return;
      }
      this.setState(this.waiting(first, attempt, lastError));
      try {
        const conn = await connectShj(this.api, this.target, {
          resume:
            this.serverId !== undefined
              ? { serverId: this.serverId, rev: this.rev }
              : undefined,
          onDiscard: this.opts.onDiscard,
        });
        if (gen !== this.generation) {
          void conn.close();
          return;
        }
        await this.adopt(conn, gen);
        return;
      } catch (e) {
        if (gen !== this.generation) return;
        if (e instanceof ShjMissingError) {
          // つなぎ直しても同じ。聞く段階へ進む（何も入れずに待つ）
          await this.askInstall(gen);
          return;
        }
        lastError = e instanceof Error ? e.message : String(e);
      }
      if (attempt >= this.opts.maxAttempts) continue; // 次の周で failed
      const d = this.opts.delays;
      const wait = d[Math.min(attempt - 1, d.length - 1)] ?? 1000;
      this.setState(
        this.waiting(first, attempt + 1, lastError, Date.now() + wait),
      );
      await this.sleep(wait);
      if (gen !== this.generation) return;
    }
  }

  /** つながった接続を受け取る。同じ serverId なら screen.get、違えば一覧から取り直す。 */
  private async adopt(conn: Connection, gen: number): Promise<void> {
    const replaced =
      this.serverId !== undefined && this.serverId !== conn.hello.serverId;
    this.conn = conn;
    this._workspaces = conn.hello.workspaces;
    this.serverId = conn.hello.serverId;
    this.rev = conn.hello.rev;
    this.fetching = false;
    let superseded = false;
    const offs = [
      conn.client.on("session.superseded", () => {
        superseded = true;
      }),
      conn.client.on("screen.update", (u) => this.handleUpdate(conn, gen, u)),
      conn.client.on("workspace.changed", (c) => {
        this._workspaces = applyWorkspaceChange(this._workspaces, c);
        this.emitWorkspaces();
      }),
      startPing(conn.client, this.opts.pingIntervalMs, this.opts.pingTimeoutMs),
    ];
    conn.client.onClose((why) => {
      offs.forEach((f) => f());
      if (gen !== this.generation || this.conn !== conn) return;
      this.conn = undefined;
      if (superseded) {
        this.setState({ kind: "superseded" });
        return;
      }
      void this.run(0, why ?? "connection ended");
    });

    this.setState({ kind: "connected", hello: conn.hello });
    this.emitWorkspaces();

    if (replaced) {
      // shj が作り直された。古い画面は捨て、ワークスペースの一覧から取り直す（画面は D 以降）
      this._screen = null;
    } else {
      await this.fetchScreen(conn, gen);
    }
  }

  private emitWorkspaces(): void {
    for (const h of [...this.workspaceHandlers]) h(this._workspaces);
  }

  private handleUpdate(conn: Connection, gen: number, u: ScreenUpdate): void {
    if (gen !== this.generation || this.conn !== conn) return;
    if (this.fetching) return; // 取り直し中の更新は、取り直した画面に含まれる
    if (u.baseRev !== this.rev) {
      void this.fetchScreen(conn, gen);
      return;
    }
    this.rev = u.rev;
    for (const h of [...this.updateHandlers]) h(u);
  }

  private async fetchScreen(conn: Connection, gen: number): Promise<void> {
    this.fetching = true;
    try {
      const screen = await conn.client.request("screen.get", {});
      if (gen !== this.generation || this.conn !== conn) return;
      this._screen = screen;
      this.rev = screen.rev;
      for (const h of [...this.screenHandlers]) h(screen);
    } catch (e) {
      // 切れたなら onClose が見る。shj が答えなかったときも ping が見る
      this.opts.onDiscard?.(
        `screen.get failed: ${e instanceof Error ? e.message : String(e)}`,
        "",
      );
    } finally {
      this.fetching = false;
    }
  }
}

/** workspace.changed を一覧に当てる。 */
export function applyWorkspaceChange(
  workspaces: HelloResult["workspaces"],
  change: Notifications["workspace.changed"],
): HelloResult["workspaces"] {
  const ws = new Map(workspaces.map((w) => [w.id, w]));
  for (const w of change.added ?? []) ws.set(w.id, w);
  for (const w of change.updated ?? []) ws.set(w.id, w);
  for (const id of change.removed ?? []) ws.delete(id);
  return [...ws.values()];
}

/**
 * session.ping を intervalMs ごとに送る。timeoutMs 経っても返事が無い（またはエラー）なら
 * 切れたとみなして client を閉じる。止める関数を返す。
 */
export function startPing(
  client: Client,
  intervalMs: number,
  timeoutMs: number,
): () => void {
  let pinging = false;
  const timer = setInterval(() => {
    if (pinging) return;
    pinging = true;
    let to: ReturnType<typeof setTimeout> | undefined;
    const timeout = new Promise<never>((_, rej) => {
      to = setTimeout(() => rej(new Error("ping timed out")), timeoutMs);
    });
    Promise.race([client.request("session.ping", {}), timeout])
      .catch((e) => client.close(e instanceof Error ? e.message : String(e)))
      .finally(() => {
        clearTimeout(to);
        pinging = false;
      });
  }, intervalMs);
  return () => clearInterval(timer);
}
