import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  absolutePath,
  cdInput,
  menuItems,
  newEntryDefault,
  newEntryPath,
  parentPath,
  renamedPath,
  shellQuote,
} from "./menu.ts";

describe("menuItems", () => {
  it("ファイルには新規作成と cd を出さない", () => {
    assert.deepEqual(menuItems({ dir: false }), [
      "copy-name",
      "copy-relative",
      "copy-absolute",
      "rename",
      "delete",
    ]);
  });
  it("フォルダには全部出す", () => {
    assert.deepEqual(menuItems({ dir: true }), [
      "copy-name",
      "copy-relative",
      "copy-absolute",
      "new-file",
      "new-dir",
      "rename",
      "delete",
      "cd",
    ]);
  });
});

describe("パス", () => {
  it("absolutePath", () => {
    assert.equal(absolutePath("/w/p", "a/b"), "/w/p/a/b");
    assert.equal(absolutePath("/w/p/", "a"), "/w/p/a");
    assert.equal(absolutePath("/w/p", ""), "/w/p");
  });
  it("parentPath", () => {
    assert.equal(parentPath("a/b/c"), "a/b");
    assert.equal(parentPath("a"), "");
  });
  it("newEntryDefault", () => {
    assert.equal(newEntryDefault(""), "");
    assert.equal(newEntryDefault("src"), "src/");
  });
  it("newEntryPath は名前が無ければ null", () => {
    assert.equal(newEntryPath(" src/a.go "), "src/a.go");
    assert.equal(newEntryPath("./a.go"), "a.go");
    assert.equal(newEntryPath("src/"), null);
    assert.equal(newEntryPath("  "), null);
  });
  it("renamedPath は変わっていなければ null", () => {
    assert.equal(renamedPath("a/b", "a/c"), "a/c");
    assert.equal(renamedPath("a/b", "a/b"), null);
    assert.equal(renamedPath("a/b", ""), null);
  });
});

describe("cdInput", () => {
  it("空白や ' を含むパスを 1 語にする", () => {
    assert.equal(shellQuote("/a b/it's"), "'/a b/it'\\''s'");
    assert.equal(cdInput("/a b"), "\x05\x15cd -- '/a b'\r");
  });
  it("親へ", () => {
    assert.equal(cdInput(), "\x05\x15cd ..\r");
  });
});
