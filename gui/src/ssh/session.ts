// 切れたら自動でつなぎ直す接続（C4）。connectShj の上に、次の 3 つを足す。
//  - session.ping を一定間隔で送り、ssh が黙って切れたことに気づく
//  - 切れたら間隔を空けて何度かつなぎ直す。session.hello で resume を送り、同じ serverId なら
//    screen.get で全部取り直す。違えばワークスペースの一覧（hello.workspaces）から取り直す
//  - 状態（State）を外に出す。一覧の表示はこれを読む
// Solid などの画面の仕組みには依存しない。

import type { HelloResult, Screen, ScreenUpdate } from "../rpc/protocol.ts";
import type { ShellApi } from "./api.ts";
import { connectShj, type Connection } from "./connect.ts";

export type SessionState =
  | { kind: "connecting"; attempt: number }
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
  /** 回数を使い切った。ボタンで retry() できる */
  | { kind: "failed"; message: string }
  /** 別の GUI に引き継がれた。自動ではつなぎ直さない（取り合いになるため。表示は C7） */
  | { kind: "superseded" }
  | { kind: "closed" };

export interface SessionOptions {
  /** つなぎ直しの間隔（ms）。n 回目の失敗の後は delays[n-1]、尽きたら最後の値。 */
  delays?: number[];
  /** 諦めるまでのつなぎ直しの回数。 */
  maxAttempts?: number;
  pingIntervalMs?: number;
  /** ping の返事を待つ時間。これを過ぎたら切れたとみなす */
  pingTimeoutMs?: number;
  onDiscard?: (reason: string, line: string) => void;
}

/** 1, 2, 4, 8, 16, 30 秒…で 10 回（合わせて 5 分ほど）。 */
export const DEFAULT_DELAYS = [1000, 2000, 4000, 8000, 16000, 30000];
export const DEFAULT_MAX_ATTEMPTS = 10;
/** 設計書 3 章：GUI は 15 秒ごとに session.ping を送る */
export const PING_INTERVAL_MS = 15000;
export const PING_TIMEOUT_MS = 10000;

export class Session {
  readonly target: string;
  private readonly api: ShellApi;
  private readonly opts: Required<Omit<SessionOptions, "onDiscard">> &
    Pick<SessionOptions, "onDiscard">;
  private _state: SessionState = { kind: "connecting", attempt: 0 };
  private conn?: Connection;
  /** 最後につながった shj の serverId と、GUI が持つ画面の版 */
  private serverId?: string;
  private rev = 0;
  private _screen: Screen | null = null;
  private _workspaces: HelloResult["workspaces"] = [];
  private timer?: ReturnType<typeof setTimeout>;
  private wake?: () => void;
  private generation = 0;
  private readonly stateHandlers = new Set<(s: SessionState) => void>();
  private readonly screenHandlers = new Set<(s: Screen) => void>();
  private readonly updateHandlers = new Set<(u: ScreenUpdate) => void>();

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

  /** 閉じる（session.bye。shj とシェルは止まらない）。 */
  async close(): Promise<void> {
    this.generation++;
    this.clearTimers();
    this.wake?.();
    const c = this.conn;
    this.conn = undefined;
    this.setState({ kind: "closed" });
    await c?.close();
  }

  private setState(s: SessionState) {
    this._state = s;
    for (const h of [...this.stateHandlers]) h(s);
  }

  private clearTimers() {
    if (this.timer) clearTimeout(this.timer);
    this.timer = undefined;
  }

  private sleep(ms: number): Promise<void> {
    return new Promise((resolve) => {
      const t = setTimeout(done, ms);
      const self = this;
      function done() {
        clearTimeout(t);
        self.wake = undefined;
        resolve();
      }
      this.wake = done;
    });
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
      if (first && attempt === 1) {
        this.setState({ kind: "connecting", attempt });
      } else {
        this.setState({
          kind: "reconnecting",
          attempt,
          max: this.opts.maxAttempts,
          reason: lastError,
        });
      }
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
        lastError = e instanceof Error ? e.message : String(e);
      }
      if (attempt >= this.opts.maxAttempts) continue; // 次の周で failed
      const d = this.opts.delays;
      const wait = d[Math.min(attempt - 1, d.length - 1)] ?? 1000;
      this.setState({
        kind: "reconnecting",
        attempt: attempt + 1,
        max: this.opts.maxAttempts,
        retryAt: Date.now() + wait,
        reason: lastError,
      });
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
    let superseded = false;
    const offs = [
      conn.client.on("session.superseded", () => {
        superseded = true;
      }),
      conn.client.on("screen.update", (u) => {
        if (gen !== this.generation) return;
        if (u.baseRev !== this.rev) {
          void this.fetchScreen(conn, gen);
          return;
        }
        this.rev = u.rev;
        for (const h of [...this.updateHandlers]) h(u);
      }),
      conn.client.on("workspace.changed", (c) => {
        const ws = new Map(this._workspaces.map((w) => [w.id, w]));
        for (const w of c.added ?? []) ws.set(w.id, w);
        for (const w of c.updated ?? []) ws.set(w.id, w);
        for (const id of c.removed ?? []) ws.delete(id);
        this._workspaces = [...ws.values()];
      }),
    ];
    let pingTimer: ReturnType<typeof setInterval> | undefined;
    const stop = () => {
      offs.forEach((f) => f());
      if (pingTimer) clearInterval(pingTimer);
    };
    conn.client.onClose((why) => {
      stop();
      if (gen !== this.generation || this.conn !== conn) return;
      this.conn = undefined;
      if (superseded) {
        this.setState({ kind: "superseded" });
        return;
      }
      void this.run(0, why ?? "connection ended");
    });

    this.setState({ kind: "connected", hello: conn.hello });

    // 黙って切れた ssh に気づく。返事が来なければ閉じて、つなぎ直しに回す
    let pinging = false;
    pingTimer = setInterval(() => {
      if (pinging) return;
      pinging = true;
      let to: ReturnType<typeof setTimeout> | undefined;
      const timeout = new Promise<never>((_, rej) => {
        to = setTimeout(
          () => rej(new Error("ping timed out")),
          this.opts.pingTimeoutMs,
        );
      });
      Promise.race([conn.client.request("session.ping", {}), timeout])
        .catch((e) => {
          conn.client.close(e instanceof Error ? e.message : String(e));
        })
        .finally(() => {
          clearTimeout(to);
          pinging = false;
        });
    }, this.opts.pingIntervalMs);

    if (replaced) {
      // shj が作り直された。古い画面は捨て、ワークスペースの一覧から取り直す（画面は D 以降）
      this._screen = null;
    } else {
      await this.fetchScreen(conn, gen);
    }
  }

  private async fetchScreen(conn: Connection, gen: number): Promise<void> {
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
    }
  }
}
