// ペインの 1 行を、等幅のマス目に並べる「区間（Run）」に分ける。色はまだテーマ上の名前のまま。
// TUI の描き方（internal/tui/editor_draw.go の lineDrawer）に合わせる：
//  - 行番号の欄（gutter）は「印の 1 桁 + 右寄せの番号 + 空き 2 桁」で、幅は全行数の桁数 + 4。
//    印の桁には、診断のある行に診断の重さの色の「●」を出す（gutterMark）。
//  - カーソルのある行は curline の背景。ただし選択があるときは付けない。
//  - 選択は selection の背景。範囲は [start, end)。行末より右も、選択が行をまたぐときは 1 桁塗る。
//  - カーソルはマス目 1 つ（タブの上では先頭の 1 桁）を cursor の色で塗り、文字を cursor.fg で描く。
//  - 横スクロール（viewport.left）より左は描かず、幅（viewport.cols）を超えたら止める。
//  - 診断（E5）：その行で始まる診断があれば、診断の範囲に重さの色の下線を引き、行末の後ろに一番重い診断の
//    文言の 1 行目を薄く出す。前の行から続く診断（cont）だけの行には何も描かない。
//  - インデントガイド（E5）：guides の桁にある空白（タブは先頭の 1 桁）に縦線を引く。indent より右には
//    引かない。空行は行末より右にも indent まで引く。縦線は文字ではなく、マス目の左端の 1px の線
//    （モックのツリーの縦線と同じ形）で描く。
// 行の文字はスタイルの区間（spans）で来るので、ここで rune（code point）に戻して数える。

import type {
  Diagnostic,
  Line,
  Pane,
  Pos,
  Range,
  Severity,
} from "../rpc/protocol.ts";
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
  /** 診断の波線の色の名前 */
  diag?: string;
  /** インデントガイドのマス目（左端に縦線を引く。1 桁で、ほかの区間とまとめない） */
  guide?: boolean;
  /** 薄く描く（行末の診断の文言） */
  faint?: boolean;
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

/** 診断の重さの色の名前（Go の core.DiagStyle）。 */
export function diagStyle(sev: Severity | number): string {
  switch (sev) {
    case 2:
      return "diag.warn";
    case 3:
      return "diag.info";
    case 4:
      return "diag.hint";
  }
  return "diag.error";
}

/** 行末に文言を出す診断。続きでない診断のうち一番重いもの（Go の core.HeadDiagnostic）。 */
export function headDiagnostic(
  ds: readonly Diagnostic[] | null | undefined,
): Diagnostic | undefined {
  let best: Diagnostic | undefined;
  for (const d of ds ?? []) {
    if (d.cont) continue;
    if (!best || d.severity < best.severity) best = d;
  }
  return best;
}

/** 行番号の横の印の色の名前。印が無ければ undefined（印の桁は空ける）。診断の印を先にする。 */
export function gutterMark(line: Line): string | undefined {
  const marks = line.marks ?? [];
  const m = marks.find((m) => m.kind === "diag") ?? marks.find((m) => m.style);
  return m?.style || undefined;
}

/** 1 つのマス目の見た目。 */
type Cell = Omit<Run, "text" | "cells"> & { ch: string; cells: number };

const LOOK = [
  "fg",
  "bg",
  "bold",
  "italic",
  "underline",
  "strike",
  "diag",
  "faint",
] as const;

function sameLook(a: Run, b: Cell): boolean {
  if (a.cursor || b.cursor || a.guide || b.guide) return false;
  return LOOK.every((k) => (a[k] || undefined) === (b[k] || undefined));
}

function runOf(c: Cell): Run {
  const r: Run = { text: c.ch, cells: c.cells };
  for (const k of [...LOOK, "cursor", "guide"] as const) {
    const v = c[k];
    if (v) (r as unknown as Record<string, unknown>)[k] = v;
  }
  return r;
}

/** 行末の診断の文言の前に空ける文字。 */
const DIAG_LEAD = "   ● ";

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
  const head = headDiagnostic(line.diag);
  const guideW = line.indent ?? 0;
  const guides = new Set(line.guides ?? []);
  const isGuideCol = (v: number) => v < guideW && guides.has(v);

  // rune ごとのスタイル
  const runes: string[] = [];
  const looks: Cell[] = [];
  for (const sp of line.spans ?? []) {
    for (const ch of sp.text) {
      runes.push(ch);
      looks.push({
        ch,
        cells: 1,
        fg: sp.style || undefined,
        bg: sp.bg || undefined,
        bold: sp.bold,
        italic: sp.italic,
        underline: sp.underline,
        strike: sp.strike,
      });
    }
  }
  const inSel = (col: number) => {
    if (row < 0) return false;
    const at = { line: row, col };
    return sels.some((s) => !before(at, s.start) && before(at, s.end));
  };
  const isCursor = (col: number) =>
    row >= 0 && curs.some((c) => c.line === row && c.col === col);
  // col に掛かる一番重い診断の色（その行で始まる診断が無い行には引かない）
  const diagAt = (col: number): string | undefined => {
    if (!head) return undefined;
    let best: Diagnostic | undefined;
    for (const d of line.diag ?? []) {
      if (
        col >= d.start &&
        col < d.end &&
        (!best || d.severity < best.severity)
      )
        best = d;
    }
    return best && diagStyle(best.severity);
  };

  const cells: Cell[] = [];
  let vis = 0;
  let drawn = 0;
  for (let col = 0; col <= runes.length && drawn < textW; col++) {
    const cursor = isCursor(col);
    const sel = inSel(col);
    const eol = col === runes.length;
    if (eol && !cursor && !sel) break;
    const ch = eol ? " " : runes[col]!;
    const rw = eol ? 1 : runeWidth(ch, vis);
    const look: Cell = eol ? { ch, cells: 1 } : { ...looks[col]! };
    if (sel) look.bg = "selection";
    if (!eol) {
      const d = diagAt(col);
      if (d) {
        look.diag = d;
        look.fg ??= d;
      }
    }
    const guide = !eol && (ch === " " || ch === "\t") && isGuideCol(vis);
    if (guide) look.fg = "indent.guide";
    if (ch === "\t" || eol) {
      // 1 桁ずつ空白にする（カーソルとガイドは先頭の桁だけ）
      for (let c = 0; c < rw; c++) {
        if (vis + c >= left && drawn < textW) {
          cells.push({
            ...look,
            ch: " ",
            cells: 1,
            cursor: cursor && c === 0,
            guide: guide && c === 0,
          });
          drawn++;
        }
      }
    } else if (vis >= left) {
      if (drawn + rw > textW) break;
      cells.push({ ...look, ch: guide ? " " : ch, cells: rw, cursor, guide });
      drawn += rw;
    } else if (vis + rw > left) {
      // 全角の文字の右半分だけが見える。空白で埋める
      const n = vis + rw - left;
      for (let c = 0; c < n; c++) cells.push({ ...look, ch: " ", cells: 1 });
      drawn += n;
    }
    vis += rw;
  }

  // 空行のガイドは行末より右にも引く
  const tailBg = inSel(runes.length) ? "selection" : undefined;
  for (let v = vis; v < guideW && drawn < textW; v++) {
    if (v < left) continue;
    const guide = isGuideCol(v);
    cells.push({
      ch: " ",
      cells: 1,
      bg: tailBg,
      fg: guide ? "indent.guide" : undefined,
      guide,
    });
    drawn++;
  }

  // 行末の診断の文言（TUI と同じく、6 桁より多く空いているときだけ）
  if (head && drawn + 6 < textW) {
    const msg = DIAG_LEAD + firstLine(head.message);
    const fg = diagStyle(head.severity);
    let v = 0;
    for (const ch of msg) {
      const w = runeWidth(ch === "\t" ? " " : ch, v);
      if (drawn + w > textW) break;
      cells.push({ ch: ch === "\t" ? " " : ch, cells: w, fg, faint: true });
      drawn += w;
      v += w;
    }
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

/** s の 1 行目。 */
function firstLine(s: string): string {
  const i = s.search(/\r?\n/);
  return i < 0 ? s : s.slice(0, i);
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
