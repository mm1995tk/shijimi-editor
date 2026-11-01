import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { Line, Pane, Pos, Range } from "../rpc/protocol.ts";
import {
  cellsFor,
  gutterText,
  gutterWidth,
  layoutLine,
  lineBg,
  paneCells,
  type Run,
} from "./layout.ts";

function pane(more: Partial<Pane> = {}): Pane {
  return {
    id: "a.go",
    title: { text: "a.go" },
    kind: "text",
    viewport: { top: 0, rows: 10, cols: 20, left: 0, total: 120 },
    curLine: 0,
    cursors: [],
    selections: [],
    lines: [],
    ...more,
  };
}

const at = (line: number, col: number): Pos => ({ line, col });
const range = (s: Pos, e: Pos): Range => ({ start: s, end: e });
const plain = (no: number, text: string): Line => ({
  no,
  spans: [{ text }],
});

/** 区間を「文字列|桁数|印」の短い形にする（読みやすく比べるため）。 */
function show(runs: Run[]): string[] {
  return runs.map((r) => {
    const marks = [r.fg, r.bg, r.cursor && "cursor", r.bold && "bold"]
      .filter(Boolean)
      .join(",");
    return `${r.text}|${r.cells}${marks ? `|${marks}` : ""}`;
  });
}

describe("gutter", () => {
  it("幅は全行数の桁数 + 4、番号は 1 始まりで右寄せ", () => {
    assert.equal(gutterWidth(120), 7);
    assert.equal(gutterWidth(9), 5);
    assert.equal(gutterText(0, 120), "    1  ");
    assert.equal(gutterText(0, 120).length, gutterWidth(120));
    assert.equal(gutterText(119, 120), "  120  ");
    assert.equal(gutterText(-1, 120), "       ");
  });

  it("テキストのペインの幅は行番号の欄を足す", () => {
    assert.equal(paneCells(pane()), 27);
    assert.equal(paneCells(pane({ kind: "preview" })), 20);
  });
});

describe("layoutLine", () => {
  it("スタイルの区間ごとに分け、色は名前のまま持つ", () => {
    const l: Line = {
      no: 0,
      spans: [
        { text: "func", style: "syntax.keyword", bold: true },
        { text: " main" },
      ],
    };
    assert.deepEqual(show(layoutLine(l, pane(), { cursor: false })), [
      "func|4|syntax.keyword,bold",
      " main|5",
    ]);
  });

  it("タブは次の 4 桁の位置まで空白にし、全角の文字は 1 文字ずつ 2 桁にする", () => {
    const l = plain(0, "a\tあい!");
    assert.deepEqual(show(layoutLine(l, pane(), { cursor: false })), [
      "a   |4",
      "あ|2",
      "い|2",
      "!|1",
    ]);
  });

  it("カーソルはマス目 1 つ。行末にあれば空白を 1 つ足す", () => {
    const p = pane({ cursors: [at(0, 1)] });
    assert.deepEqual(show(layoutLine(plain(0, "abc"), p, { cursor: true })), [
      "a|1",
      "b|1|cursor",
      "c|1",
    ]);
    const end = pane({ cursors: [at(0, 3)] });
    assert.deepEqual(show(layoutLine(plain(0, "abc"), end, { cursor: true })), [
      "abc|3",
      " |1|cursor",
    ]);
    // キーを受けていないペインには描かない
    assert.deepEqual(show(layoutLine(plain(0, "abc"), p, { cursor: false })), [
      "abc|3",
    ]);
    // 別の行のカーソルは描かない
    assert.deepEqual(show(layoutLine(plain(1, "abc"), p, { cursor: true })), [
      "abc|3",
    ]);
  });

  it("タブの上のカーソルは先頭の 1 桁だけ", () => {
    const p = pane({ cursors: [at(0, 0)] });
    assert.deepEqual(show(layoutLine(plain(0, "\tx"), p, { cursor: true })), [
      " |1|cursor",
      "   x|4",
    ]);
  });

  it("選択は [start, end) を塗り、行をまたぐときは行末の 1 桁も塗る", () => {
    const p = pane({ selections: [range(at(0, 1), at(1, 2))] });
    assert.deepEqual(show(layoutLine(plain(0, "abc"), p, { cursor: false })), [
      "a|1",
      "bc |3|selection",
    ]);
    assert.deepEqual(show(layoutLine(plain(1, "xyz"), p, { cursor: false })), [
      "xy|2|selection",
      "z|1",
    ]);
    assert.deepEqual(show(layoutLine(plain(2, "xyz"), p, { cursor: false })), [
      "xyz|3",
    ]);
  });

  it("複数のカーソルと選択", () => {
    const p = pane({
      cursors: [at(0, 0), at(0, 4)],
      selections: [range(at(0, 0), at(0, 0)), range(at(0, 3), at(0, 4))],
    });
    assert.deepEqual(
      show(layoutLine(plain(0, "abcdef"), p, { cursor: true })),
      ["a|1|cursor", "bc|2", "d|1|selection", "e|1|cursor", "f|1"],
    );
  });

  it("横スクロールより左は描かず、幅を超えたら止める", () => {
    const p = pane({
      viewport: { top: 0, rows: 10, cols: 4, left: 2, total: 9 },
    });
    assert.deepEqual(
      show(layoutLine(plain(0, "abcdefgh"), p, { cursor: false })),
      ["cdef|4"],
    );
    // 全角の文字の右半分だけが見えるときは空白、入りきらない全角の文字は描かない
    const q = pane({
      viewport: { top: 0, rows: 10, cols: 4, left: 1, total: 9 },
    });
    assert.deepEqual(
      show(layoutLine(plain(0, "あいう"), q, { cursor: false })),
      [" |1", "い|2"],
    );
  });

  it("テキスト以外のペインにはカーソルと選択を描かない", () => {
    const p = pane({
      kind: "preview",
      cursors: [at(0, 0)],
      selections: [range(at(0, 0), at(0, 2))],
    });
    assert.deepEqual(show(layoutLine(plain(0, "ab"), p, { cursor: true })), [
      "ab|2",
    ]);
  });

  it("span の背景は選択より弱い", () => {
    const l: Line = { no: 0, spans: [{ text: "ab", bg: "md.code.bg" }] };
    const p = pane({ selections: [range(at(0, 1), at(0, 2))] });
    assert.deepEqual(show(layoutLine(l, p, { cursor: false })), [
      "a|1|md.code.bg",
      "b|1|selection",
    ]);
  });
});

describe("lineBg", () => {
  it("カーソルのある行は curline。選択があるときは付けない", () => {
    const p = pane({ curLine: 3 });
    assert.equal(lineBg(plain(3, ""), p), "curline");
    assert.equal(lineBg(plain(2, ""), p), undefined);
    const sel = pane({
      curLine: 3,
      selections: [range(at(3, 0), at(3, 1))],
    });
    assert.equal(lineBg(plain(3, ""), sel), undefined);
    assert.equal(
      lineBg(plain(3, ""), pane({ kind: "diff", curLine: 3 })),
      undefined,
    );
  });
});

describe("cellsFor", () => {
  it("マス目の幅と行の高さで割り、少なくとも 1 にする", () => {
    assert.deepEqual(cellsFor(800, 20 + 22 * 30 + 5, 8), {
      cols: 100,
      rows: 30,
    });
    assert.deepEqual(cellsFor(0, 0, 8), { cols: 1, rows: 1 });
  });
});
