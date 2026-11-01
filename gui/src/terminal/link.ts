// 1 つのマシンのターミナル 1 つと、それを描く画面（xterm.js）をつなぐ（F1）。
// 設計は docs/design/gui-protocol.md の 3 章「ターミナル」。
//
// - term.output は seq（バイト数の累計。そのかたまりの終わり）の順に画面へ書き、書き終えたら term.ack で返す。
//   shj は ack から 256 KiB より先を送らないので、画面が遅れると shj が待ち、`yes` のような大量の出力でも
//   画面が固まらない。
// - seq が飛んだ（溢れて捨てられた）とき・つなぎ直したときは、term.attach で溜めた出力を取り直す。
//   attach の返事を待つ間に届いた term.output は、取り直す分に入っているので捨てる。
// - term.create の返事より先に term.output が届くことがあるので、作ったら term.attach（0 から）で取り直す。
//
// DOM を使わないので node --test で確かめられる。画面は TermSink、shj は TermPort で差し替える。

import { ErrorCode, RpcError } from "../rpc/message.ts";
import type { Requests, TermId } from "../rpc/protocol.ts";
import type { TermEvent, TermMethod } from "../ssh/session.ts";

/** shj とのやりとり（Session が持つ）。 */
export interface TermPort {
  request<M extends TermMethod>(
    method: M,
    params: Requests[M]["params"],
  ): Promise<Requests[M]["result"]>;
  ackTerm(termId: TermId, seq: number): void;
}

/** 描く先（xterm.js）。 */
export interface TermSink {
  /** バイト列を書く。書き終えたら done を呼ぶ（xterm.js の write のコールバック）。 */
  write(data: Uint8Array, done: () => void): void;
  /** 知らせ（シェルの出力ではない文字）を書く。 */
  notice(text: string): void;
  /** 中身を消す（別のターミナルを描き始めるとき）。 */
  reset(): void;
}

/** 大きさが分からないうちに作るときの大きさ。 */
export const DEFAULT_SIZE = { cols: 80, rows: 24 };

export const EXITED_NOTICE =
  "\r\n[シェルが終わりました。Enter で新しいシェルを開きます]\r\n";
export const RECREATED_NOTICE =
  "\r\n[前のシェルが見つからないので、新しいシェルを開きました]\r\n";

export class TermLink {
  private readonly port: TermPort;
  private readonly onError: (message: string) => void;
  private sink?: TermSink;
  private termId?: TermId;
  /** 画面に渡した分の seq（バイト数の累計） */
  private received = 0;
  /** attach の返事待ち。この間の term.output は捨てる */
  private attaching = false;
  private exited = false;
  private cols = DEFAULT_SIZE.cols;
  private rows = DEFAULT_SIZE.rows;
  /** 続けて始めたとき、古い返事を捨てるための番号 */
  private op = 0;

  constructor(port: TermPort, onError: (message: string) => void = () => {}) {
    this.port = port;
    this.onError = onError;
  }

  /** 今描いているターミナル（まだ無ければ undefined）。 */
  get id(): TermId | undefined {
    return this.termId;
  }

  /**
   * 画面をつなぐ。前に使っていたターミナルがあれば溜めた出力を全部取り直して続け、無ければ
   * 動いているものにつなぐか新しく作る。つながっていないときは、つないだあと reconnected() で始まる。
   */
  bind(sink: TermSink): void {
    this.sink = sink;
    sink.reset();
    this.received = 0;
    void this.open(0);
  }

  /** 画面を外す（畳んだ・別のマシンを出した）。シェルは止めない。 */
  unbind(): void {
    this.sink = undefined;
    this.op++;
    this.attaching = false;
  }

  /** つなぎ直した。続きから取り直す（shj が作り直されていれば新しく作る）。 */
  reconnected(): void {
    if (this.sink) void this.open(this.received);
  }

  /** 画面の大きさが変わった。 */
  resize(cols: number, rows: number): void {
    if (cols <= 0 || rows <= 0) return;
    if (cols === this.cols && rows === this.rows) return;
    this.cols = cols;
    this.rows = rows;
    const id = this.termId;
    if (id === undefined || this.exited) return;
    this.port
      .request("term.resize", { termId: id, cols, rows })
      .catch((e) => this.onError(`term.resize: ${message(e)}`));
  }

  /** キー入力（xterm.js が作ったバイト列）。シェルが終わったあとの Enter で新しいシェルを開く。 */
  input(data: Uint8Array): void {
    const id = this.termId;
    if (this.exited) {
      if (data.includes(13)) void this.restart();
      return;
    }
    if (id === undefined || this.attaching) return;
    this.port
      .request("term.input", { termId: id, data: encodeBase64(data) })
      .catch((e) => this.onError(`term.input: ${message(e)}`));
  }

  /** shj からの通知。 */
  handle(e: TermEvent): void {
    if (e.params.termId !== this.termId || !this.sink) return;
    switch (e.method) {
      case "term.output":
        this.output(e.params.seq, decodeBase64(e.params.data));
        return;
      case "term.exit":
        // 送り残した出力より先に届くことがあるので、残りを取ってから知らせる
        this.exited = true;
        void this.attach(this.op, this.received).then((r) => {
          if (r === "ok" && this.exited) this.sink?.notice(EXITED_NOTICE);
        });
        return;
      case "term.title":
        return; // F1 ではタイトルを出す場所が無い
    }
  }

  private output(seq: number, data: Uint8Array): void {
    if (this.attaching || seq <= this.received) return;
    const start = seq - data.length;
    if (start > this.received) {
      // 溢れて捨てられた分がある。残っている分を取り直す
      void this.attach(this.op, this.received);
      return;
    }
    this.write(data.subarray(this.received - start), seq);
  }

  private write(data: Uint8Array, seq: number): void {
    const sink = this.sink;
    const id = this.termId;
    if (!sink || id === undefined) return;
    this.received = seq;
    const op = this.op;
    sink.write(data, () => {
      // 書き終えた分を返す（別のターミナルに移っていたら返さない）
      if (op === this.op && id === this.termId) this.port.ackTerm(id, seq);
    });
  }

  /** 前のターミナルがあればそれにつなぎ、無ければ動いているものか新しいものにつなぐ。 */
  private async open(fromSeq: number): Promise<void> {
    const op = ++this.op;
    if (this.termId !== undefined) {
      const r = await this.attach(op, fromSeq);
      if (op !== this.op) return;
      // 描き直した画面にだけ、終わったことをもう一度出す
      if (r === "ok" && this.exited && fromSeq === 0)
        this.sink?.notice(EXITED_NOTICE);
      if (r !== "missing") return;
      // shj が作り直された（not_found）。新しく作る
      this.termId = undefined;
      this.received = 0;
      this.exited = false;
      this.sink?.notice(RECREATED_NOTICE);
    }
    try {
      const { terms } = await this.port.request("term.list", {});
      if (op !== this.op) return;
      const alive = terms.find((t) => t.alive);
      if (alive) {
        this.termId = alive.termId;
        this.exited = false;
        await this.attach(op, 0);
        if (op === this.op) this.sendSize(alive.cols, alive.rows);
        return;
      }
    } catch (e) {
      if (op !== this.op) return;
      this.onError(`term.list: ${message(e)}`);
      return;
    }
    await this.create(op);
  }

  private async create(op: number): Promise<void> {
    try {
      const r = await this.port.request("term.create", {
        cols: this.cols,
        rows: this.rows,
      });
      if (op !== this.op) return;
      this.termId = r.termId;
      this.exited = false;
      this.received = 0;
      await this.attach(op, 0);
    } catch (e) {
      if (op === this.op) this.onError(`term.create: ${message(e)}`);
    }
  }

  /** シェルが終わったあと。古いものを閉じて新しく作る。 */
  private async restart(): Promise<void> {
    const old = this.termId;
    const op = ++this.op;
    this.termId = undefined;
    this.exited = false;
    this.received = 0;
    this.sink?.reset();
    if (old !== undefined)
      this.port.request("term.close", { termId: old }).catch(() => {});
    await this.create(op);
  }

  /**
   * fromSeq から溜めた出力を取り直して書く。ターミナルが無ければ "missing"、ほかの失敗と古い返事は "failed"。
   * 取り直した分は shj が受け取ったものとみなすので、ack は返さない。
   */
  private async attach(
    op: number,
    fromSeq: number,
  ): Promise<"ok" | "missing" | "failed"> {
    const id = this.termId;
    if (id === undefined) return "missing";
    this.attaching = true;
    try {
      const r = await this.port.request("term.attach", {
        termId: id,
        fromSeq,
      });
      if (op !== this.op || id !== this.termId) return "failed";
      this.attaching = false;
      const data = decodeBase64(r.replay);
      this.received = r.seq;
      if (data.length > 0) this.sink?.write(data, () => {});
      return "ok";
    } catch (e) {
      if (op !== this.op || id !== this.termId) return "failed";
      this.attaching = false;
      if (e instanceof RpcError && e.code === ErrorCode.NotFound)
        return "missing";
      this.onError(`term.attach: ${message(e)}`);
      return "failed";
    }
  }

  /** つないだターミナルの大きさが画面と違えば合わせる。 */
  private sendSize(cols: number, rows: number): void {
    const id = this.termId;
    if (id === undefined || (cols === this.cols && rows === this.rows)) return;
    this.port
      .request("term.resize", { termId: id, cols: this.cols, rows: this.rows })
      .catch((e) => this.onError(`term.resize: ${message(e)}`));
  }
}

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

export function encodeBase64(data: Uint8Array): string {
  let s = "";
  for (let i = 0; i < data.length; i += 0x8000)
    s += String.fromCharCode(...data.subarray(i, i + 0x8000));
  return btoa(s);
}

export function decodeBase64(b64: string): Uint8Array {
  const s = atob(b64);
  const out = new Uint8Array(s.length);
  for (let i = 0; i < s.length; i++) out[i] = s.charCodeAt(i);
  return out;
}
