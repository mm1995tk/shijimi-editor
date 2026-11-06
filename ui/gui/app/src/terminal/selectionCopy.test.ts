import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { copyAndNotify, SelectionCopy } from "./selectionCopy.ts";

describe("SelectionCopy", () => {
  it("押している間に選択が変わったら、離したときにコピーする", () => {
    const s = new SelectionCopy();
    s.press();
    s.selectionChanged();
    assert.equal(s.release(), true);
  });

  it("クリックだけ（選択が変わらない）ならコピーしない", () => {
    const s = new SelectionCopy();
    s.press();
    assert.equal(s.release(), false);
  });

  it("押していないときの選択の変化と離しはコピーしない", () => {
    const s = new SelectionCopy();
    s.selectionChanged();
    assert.equal(s.release(), false);
  });

  it("前の選択の変化を引きずらない", () => {
    const s = new SelectionCopy();
    s.press();
    s.selectionChanged();
    s.release();
    s.press();
    assert.equal(s.release(), false);
  });
});

describe("copyAndNotify", () => {
  it("書けたら知らせる", async () => {
    let n = 0;
    await copyAndNotify(
      async () => {},
      () => n++,
    );
    assert.equal(n, 1);
  });

  it("書けなかったら知らせない", async () => {
    let n = 0;
    await copyAndNotify(
      async () => {
        throw new Error("denied");
      },
      () => n++,
    );
    assert.equal(n, 0);
  });
});
