// 1 つのマシンのターミナルを並べて持つ（F4）。設計書 docs/design/gui-protocol.md の 6 章 F の表の F4。
//
// - ターミナルモードでは、フォーカスしているマシンのターミナルを全部（6 個まで）並べる。IDE モードの右下に
//   出すのは、そのうちの 1 つ（今使っているもの = active）。並べ方は grid.ts。
// - 初めて出すときに term.list で動いているシェルを拾って並べ、1 つも無ければ 1 つ作る。
// - Ctrl+Shift+T で増やす（add）。シェルが終わったら（`exit`）その欄を閉じて並び直す。最後の 1 つだけは
//   閉じず、F1 と同じく知らせを出して Enter で作り直せるようにする（ターミナルの欄を空にしない）。
// - ワークスペースを移したら（F2）、そのワークスペースで最後に使ったシェルを active にする。並べていなければ
//   足し、そのワークスペースにシェルが無ければルートで作る。
// - ターミナル 1 つの流量制御・取り直しは TermLink（link.ts）がする。
//
// DOM を使わないので node --test で確かめられる。

import type { TermId } from "../rpc/protocol.ts";
import type { TermEvent } from "../ssh/session.ts";
import { MAX_TERMS, neighbor, type Direction } from "./grid.ts";
import { TermLink, type TermPort } from "./link.ts";

/**
 * 1 つのマシンのターミナルの並びと、今使っている 1 つ（active）を持つ。並びか active が変わったら onChange を呼ぶ。
 * 出す前は何もせず、show() で初めて動いているシェルを拾う。決まりはこのファイルの先頭のとおり。
 */
export class TermGroup {
  private readonly port: TermPort;
  private readonly onChange: () => void;
  private readonly onError: (message: string) => void;
  private links: TermLink[] = [];
  private current?: TermLink;
  /** 画面に出しているか。出す前は term.list も term.create もしない */
  private shown = false;
  /** term.list で動いているシェルを拾ったか */
  private populated = false;
  /** 続けて拾い始めたとき、古い返事を捨てるための番号 */
  private loadSeq = 0;

  constructor(
    port: TermPort,
    onChange: () => void,
    onError: (message: string) => void = () => {},
  ) {
    this.port = port;
    this.onChange = onChange;
    this.onError = onError;
  }

  /** 並べるターミナル（並べる順）。変えるたびに新しい配列にする。 */
  get list(): readonly TermLink[] {
    return this.links;
  }

  /** 今使っているターミナル（IDE モードの右下に出すもの・キーを向けるもの）。 */
  get active(): TermLink | undefined {
    return this.current;
  }

  /** 画面に出した。初めてなら動いているシェルを拾う（無ければ作る）。 */
  show(): void {
    this.shown = true;
    if (!this.populated) void this.populate();
  }

  /** 画面から外した（別のマシンを出した）。シェルは止めない。 */
  hide(): void {
    this.shown = false;
    this.loadSeq++;
  }

  /** つなぎ直した。拾う前なら拾い、拾ったあとならそれぞれ続きから取り直す。 */
  reconnected(): void {
    if (!this.shown) return;
    if (!this.populated) void this.populate();
    for (const l of this.links) l.reconnected();
  }

  /** shj からの通知。termId が合うものが受ける。 */
  handle(e: TermEvent): void {
    for (const l of this.links) l.handle(e);
  }

  /**
   * 新しいターミナルを足して active にする（Ctrl+Shift+T）。cwd が無ければ shj に任せる
   * （フォーカスしているワークスペースのルート、それも無ければ ~）。上限なら足さずに false。
   */
  add(cwd?: string): boolean {
    this.pruneExited();
    if (this.links.length >= MAX_TERMS) return false;
    const l = this.newLink();
    if (cwd !== undefined) l.switchTo(undefined, cwd);
    this.links = [...this.links, l];
    this.current = l;
    this.onChange();
    return true;
  }

  /** i 番目を active にする（押した欄）。 */
  setActive(i: number): void {
    const l = this.links[i];
    if (!l || l === this.current) return;
    this.current = l;
    this.onChange();
  }

  /** active を dir の向きの隣へ移す（Ctrl+Shift+矢印）。動いたら true。 */
  move(dir: Direction): boolean {
    const i = this.current ? this.links.indexOf(this.current) : -1;
    if (i < 0) return false;
    const j = neighbor(i, this.links.length, dir);
    if (j === i) return false;
    this.current = this.links[j];
    this.onChange();
    return true;
  }

  /**
   * ワークスペースを移した（F2）。termId（そのワークスペースで最後に使ったシェル）を active にする。
   * 並べていなければ足し、termId が無ければ cwd（そのワークスペースのルート）で作る。
   * 上限まで並べているときは、active の欄をそのシェルに切り替える（前のシェルは止めない）。
   */
  switchTo(termId: TermId | undefined, cwd: string): void {
    const found =
      termId !== undefined
        ? this.links.find((l) => l.id === termId)
        : undefined;
    if (found) {
      this.setActive(this.links.indexOf(found));
      return;
    }
    this.pruneExited();
    if (this.links.length < MAX_TERMS || !this.current) {
      const l = this.newLink();
      l.switchTo(termId, cwd);
      this.links = [...this.links, l];
      this.current = l;
    } else {
      this.current.switchTo(termId, cwd);
    }
    this.onChange();
  }

  private newLink(): TermLink {
    const l = new TermLink(this.port, this.onError, { adopt: false });
    l.onExit = () => this.exited(l);
    return l;
  }

  /** term.list で動いているシェルを拾って前に並べる。並べるものが 1 つも無ければ 1 つ作る。 */
  private async populate(): Promise<void> {
    const seq = ++this.loadSeq;
    let terms;
    try {
      ({ terms } = await this.port.request("term.list", {}));
    } catch (e) {
      // つながる前なら、つないだあと reconnected() でもう一度拾う
      if (seq === this.loadSeq)
        this.onError(`term.list: ${e instanceof Error ? e.message : e}`);
      return;
    }
    if (seq !== this.loadSeq || !this.shown) return;
    this.populated = true;
    const known = new Set(this.links.map((l) => l.id));
    const adopted = terms
      .filter((t) => t.alive && !known.has(t.termId))
      .slice(0, Math.max(0, MAX_TERMS - this.links.length))
      .map((t) => {
        const l = this.newLink();
        // 作り直すとき（shj が作り直された）は、そのシェルがいたフォルダで作る
        l.switchTo(t.termId, t.cwd);
        return l;
      });
    this.links = [...adopted, ...this.links];
    if (this.links.length === 0) this.links = [this.newLink()];
    this.current ??= this.links[0];
    this.onChange();
  }

  /** シェルが終わった。ほかにもあれば欄を閉じて並び直す。最後の 1 つなら残す（Enter で作り直す）。 */
  private exited(l: TermLink): void {
    const i = this.links.indexOf(l);
    if (i < 0 || this.links.length <= 1) return;
    this.remove(l);
    this.onChange();
  }

  /** 終わったまま残っている欄（最後の 1 つだった）を閉じる。足す前に使う。 */
  private pruneExited(): void {
    for (const l of this.links) if (l.hasExited) this.remove(l);
  }

  private remove(l: TermLink): void {
    const i = this.links.indexOf(l);
    if (i < 0) return;
    this.links = this.links.filter((x) => x !== l);
    l.onExit = undefined;
    if (l.id !== undefined)
      this.port.request("term.close", { termId: l.id }).catch(() => {});
    if (this.current === l)
      this.current = this.links[Math.min(i, this.links.length - 1)];
  }
}
