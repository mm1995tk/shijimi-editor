// ペインの 1 行を、等幅のマス目に並べる「区間（Run）」に分ける。色はまだテーマ上の名前のまま。
// TUI の描き方（internal/tui/editor_draw.go の lineDrawer）に合わせる：
//  - 行番号の欄（gutter）は「印の 1 桁 + 右寄せの番号 + 空き 2 桁」で、幅は全行数の桁数 + 4。
//  - カーソルのある行は curline の背景。ただし選択があるときは付けない。
//  - 選択は selection の背景。範囲は [start, end)。行末より右も、選択が行をまたぐときは 1 桁塗る。
//  - カーソルはマス目 1 つ（タブの上では先頭の 1 桁）を cursor の色で塗り、文字を cursor.fg で描く。
//  - 横スクロール（viewport.left）より左は描かず、幅（viewport.cols）を超えたら止める。
// 行の文字はスタイルの区間（spans）で来るので、ここで rune（code point）に戻して数える。

import type { Line, Pane, Pos, Range, Span } from "../rpc/protocol.ts";
import { runeWidth } from "./width.ts";

/** 同じ見た目で続くマス目の区間。 */
export interface Run {
  text: string;
  /** 使うマス目の数（全角 1 文字なら 2） */
  cells: number;
  /** 文字の色の名前。undefined は標準の文字色（editor.fg） */
  fg?: string;
  /** 背景の色の名前。undefined は行の背景 */
  bg?: string;
  bold?: boolean;
  italic?: boolean;
  underline?: boolean;
  strike?: boolean;
  /** カーソルのマス目 */
  cursor?: boolean;
}

/** 描き方の選択。 */
export interface LineOptions {
  /** カーソルを描くか（キーを受けているペインだけ） */
  cursor: boolean;
}

/** 行番号の欄の幅（マス目）。shj の core.GutterWidthFor と同じ。 */
export function gutterWidth(total: number): number {
  return String(Math.max(0, total)).length + 4;
}

/** 行番号の欄の文字（印の 1 桁は空き）。no はバッファの行番号（0 始まり）。 */
export function gutterText(no: number, total: number): string {
  if (no < 0) return " ".repeat(gutterWidth(total));
  return ` ${String(no + 1).padStart(gutterWidth(total) - 3)}  `;
}

/** ペインの横幅（マス目）。テキストは行番号の欄を足す。 */
export function paneCells(p: Pane): number {
  const cols = Math.max(0, p.viewport.cols);
  return p.kind === "text" ? cols + gutterWidth(p.viewport.total) : cols;
}

/** 行の背景の名前。カーソルのある行は curline（選択があるときは付けない）。 */
export function lineBg(line: Line, p: Pane): string | undefined {
  if (line.band) return line.band;
  if (p.kind !== "text" || line.no < 0 || line.no !== p.curLine)
    return undefined;
  return hasSelection(p.selections) ? undefined : "curline";
}

function hasSelection(sel: readonly Range[] | null | undefined): boolean {
  const s = sel?.[0];
  return !!s && (s.start.line !== s.end.line || s.start.col !== s.end.col);
}

function before(a: Pos, b: Pos): boolean {
  return a.line < b.line || (a.line === b.line && a.col < b.col);
}

/** 1 つのマス目（rune 1 つ）の見た目。 */
interface Cell {
  ch: string;
  cells: number;
  span?: Span;
  bg?: string;
  cursor?: boolean;
}

function sameLook(a: Run, b: Cell): boolean {
  const s = b.span;
  return (
    !a.cursor &&
    !b.cursor &&
    a.fg === (s?.style || undefined) &&
    a.bg === b.bg &&
    !!a.bold === !!s?.bold &&
    !!a.italic === !!s?.italic &&
    !!a.underline === !!s?.underline &&
    !!a.strike === !!s?.strike
  );
}

function runOf(c: Cell): Run {
  const s = c.span;
  const r: Run = { text: c.ch, cells: c.cells };
  if (s?.style) r.fg = s.style;
  if (c.bg) r.bg = c.bg;
  if (s?.bold) r.bold = true;
  if (s?.italic) r.italic = true;
  if (s?.underline) r.underline = true;
  if (s?.strike) r.strike = true;
  if (c.cursor) r.cursor = true;
  return r;
}

/**
 * line（p.lines の 1 つ）を、p の横スクロールと幅に合わせた区間の並びにする。
 * 全角の文字は、描く側が幅をそろえられるよう 1 文字ずつ別の区間にする。
 */
export function layoutLine(line: Line, p: Pane, o: LineOptions): Run[] {
  const row = line.no;
  const textW = Math.max(0, p.viewport.cols);
  const left = Math.max(0, p.viewport.left);
  const text = p.kind === "text";
  const sels = text ? (p.selections ?? []) : [];
  const curs = text && o.cursor ? (p.cursors ?? []) : [];

  // rune ごとのスタイル
  const runes: string[] = [];
  const spanOf: Span[] = [];
  for (const sp of line.spans ?? []) {
    for (const ch of sp.text) {
      runes.push(ch);
      spanOf.push(sp);
    }
  }
  const inSel = (col: number) => {
    if (row < 0) return false;
    const at = { line: row, col };
    return sels.some((s) => !before(at, s.start) && before(at, s.end));
  };
  const isCursor = (col: number) =>
    row >= 0 && curs.some((c) => c.line === row && c.col === col);

  const cells: Cell[] = [];
  let vis = 0;
  let drawn = 0;
  for (let col = 0; col <= runes.length && drawn < textW; col++) {
    const cursor = isCursor(col);
    const sel = inSel(col);
    const eol = col === runes.length;
    if (eol && !cursor && !sel) break;
    const ch = eol ? " " : runes[col]!;
    const span = eol ? undefined : spanOf[col];
    const rw = eol ? 1 : runeWidth(ch, vis);
    const bg = sel ? "selection" : span?.bg || undefined;
    if (ch === "\t" || eol) {
      // 1 桁ずつ空白にする（カーソルは先頭の桁だけ）
      for (let c = 0; c < rw; c++) {
        if (vis + c >= left && drawn < textW) {
          cells.push({
            ch: " ",
            cells: 1,
            span,
            bg,
            cursor: cursor && c === 0,
          });
          drawn++;
        }
      }
    } else if (vis >= left) {
      if (drawn + rw > textW) break;
      cells.push({ ch, cells: rw, span, bg, cursor });
      drawn += rw;
    } else if (vis + rw > left) {
      // 全角の文字の右半分だけが見える。空白で埋める
      const n = vis + rw - left;
      for (let c = 0; c < n; c++) cells.push({ ch: " ", cells: 1, span, bg });
      drawn += n;
    }
    vis += rw;
  }

  // 同じ見た目の 1 桁の文字をまとめる
  const out: Run[] = [];
  let narrow = false; // 最後の区間が 1 桁の文字だけでできている
  for (const c of cells) {
    const last = out.at(-1);
    if (last && narrow && c.cells === 1 && sameLook(last, c)) {
      last.text += c.ch;
      last.cells++;
      continue;
    }
    out.push(runOf(c));
    narrow = c.cells === 1;
  }
  return out;
}

/** 1 行の高さ（px）。files.css の --files-line-height と同じ。 */
export const LINE_HEIGHT = 22;
/** 本文の上下の余白（px）。files.css の --files-pad-y と同じ。 */
export const PAD_Y = 10;

/** 欄に入るマス目の数。 */
export interface FilesSize {
  cols: number;
  rows: number;
}

/** 本文の px の大きさ（余白を含む）から、入るマス目の桁数と行数を出す（少なくとも 1）。 */
export function cellsFor(
  width: number,
  height: number,
  cellWidth: number,
): FilesSize {
  return {
    cols: Math.max(1, Math.floor(width / Math.max(1, cellWidth))),
    rows: Math.max(1, Math.floor((height - 2 * PAD_Y) / LINE_HEIGHT)),
  };
}

/**
 * line を描くのに使うペインの項目だけを並べた文字列。これが同じなら layoutLine と lineBg の結果も同じなので、
 * 描く側はペインが新しいオブジェクトになっても（カーソルが別の行で動いたなど）、この行を描き直さずに済む。
 * line そのものは含めない（行が変われば、描く側は別の行として作り直す）。
 */
export function lineKey(line: Line, p: Pane, o: LineOptions): string {
  const row = line.no;
  const text = p.kind === "text";
  const touches = (s: Range) => s.start.line <= row && row <= s.end.line;
  return JSON.stringify([
    p.kind,
    p.viewport.cols,
    p.viewport.left,
    p.viewport.total,
    row >= 0 && row === p.curLine,
    hasSelection(p.selections),
    text && o.cursor
      ? (p.cursors ?? []).filter((c) => c.line === row).map((c) => c.col)
      : [],
    text ? (p.selections ?? []).filter(touches) : [],
  ]);
}
