import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { runeWidth, visualCol } from "./width.ts";

describe("runeWidth", () => {
  it("ASCII と曖昧な幅の文字は 1 桁、全角と絵文字は 2 桁", () => {
    assert.equal(runeWidth("a", 0), 1);
    assert.equal(runeWidth("é", 0), 1);
    assert.equal(runeWidth("─", 0), 1);
    assert.equal(runeWidth("あ", 0), 2);
    assert.equal(runeWidth("漢", 0), 2);
    assert.equal(runeWidth("한", 0), 2);
    assert.equal(runeWidth("Ａ", 0), 2);
    assert.equal(runeWidth("😀", 0), 2);
    assert.equal(runeWidth("𠀋", 0), 2);
  });

  it("幅 0 の文字も 1 桁に数える（shj の buffer.RuneWidth と同じ）", () => {
    assert.equal(runeWidth("́", 0), 1);
    assert.equal(runeWidth("​", 0), 1);
  });

  it("タブは 4 桁ごとの位置まで進む", () => {
    assert.equal(runeWidth("\t", 0), 4);
    assert.equal(runeWidth("\t", 1), 3);
    assert.equal(runeWidth("\t", 4), 4);
    assert.equal(runeWidth("\t", 7), 1);
  });
});

describe("visualCol", () => {
  it("rune の位置より前の幅を足す", () => {
    const runes = [..."a\tあb"];
    assert.equal(visualCol(runes, 0), 0);
    assert.equal(visualCol(runes, 1), 1);
    assert.equal(visualCol(runes, 2), 4);
    assert.equal(visualCol(runes, 3), 6);
    assert.equal(visualCol(runes, 99), 7);
  });
});
