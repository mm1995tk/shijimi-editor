import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { Workspace } from "../rpc/protocol.ts";
import { LOCAL } from "../ssh/api.ts";
import { adoptFocus, band, buildSidebar, health, keepFocus } from "./model.ts";

const ws = (
  w: Partial<Workspace> & { id: string; root: string },
): Workspace => ({
  kind: "git",
  name: w.root.split("/").at(-1) ?? "",
  terms: [],
  files: [],
  ...w,
});

describe("band", () => {
  it("git のワークスペースは名前とブランチ", () => {
    const b = band(
      ws({ id: "a", root: "/src/shijimi-editor", branch: "main" }),
      false,
    );
    assert.equal(b.title, "shijimi-editor");
    assert.equal(b.detail, "main");
    assert.equal(b.path, false);
    assert.equal(b.root, "/src/shijimi-editor");
  });

  it("目印が無いワークスペースはルートのパスと「git なし」", () => {
    const b = band(
      ws({ id: "n", root: "/etc/nginx", kind: "cwd", name: "nginx" }),
      true,
    );
    assert.equal(b.title, "/etc/nginx");
    assert.equal(b.path, true);
    assert.equal(b.detail, "git なし");
    assert.equal(b.focused, true);
  });

  it("ブランチが無ければ右は空", () => {
    const b = band(ws({ id: "s", root: "/x/notes", kind: "shijimi" }), false);
    assert.equal(b.detail, "");
  });
});

describe("health", () => {
  it("状態を 3 つに分ける", () => {
    assert.equal(health({ kind: "connected", hello: {} as never }), "up");
    assert.equal(
      health({ kind: "reconnecting", attempt: 1, max: 3, reason: "" }),
      "pending",
    );
    assert.equal(health({ kind: "failed", message: "x" }), "down");
    assert.equal(health(undefined), "down");
  });
});

describe("buildSidebar", () => {
  it("つなぎ先の順に見出しを並べ、帯はルートの順、フォーカス中の帯に印を付ける", () => {
    const groups = buildSidebar(
      [LOCAL, "dev-vm"],
      { "dev-vm": { kind: "connected", hello: {} as never } },
      {
        "dev-vm": [
          ws({ id: "p", root: "/src/platform" }),
          ws({ id: "s", root: "/src/app" }),
        ],
      },
      { target: "dev-vm", id: "p" },
    );
    assert.deepEqual(
      groups.map((g) => [g.label, g.health]),
      [
        ["このマシン", "down"],
        ["dev-vm", "up"],
      ],
    );
    assert.deepEqual(groups[0].bands, []);
    assert.deepEqual(
      groups[1].bands.map((b) => [b.id, b.focused]),
      [
        ["s", false],
        ["p", true],
      ],
    );
  });

  it("同じ id でも別のマシンの帯には印を付けない", () => {
    const groups = buildSidebar(
      ["a", "b"],
      {},
      { a: [ws({ id: "x", root: "/x" })], b: [ws({ id: "x", root: "/x" })] },
      { target: "b", id: "x" },
    );
    assert.equal(groups[0].bands[0].focused, false);
    assert.equal(groups[1].bands[0].focused, true);
  });
});

describe("adoptFocus", () => {
  it("フォーカスが無ければ、先に分かったマシンのものを使う", () => {
    assert.deepEqual(adoptFocus(null, "a", "x"), { target: "a", id: "x" });
    assert.equal(adoptFocus(null, "a", null), null);
  });

  it("別のマシンのフォーカスでは動かさない", () => {
    const cur = { target: "a", id: "x" };
    assert.equal(adoptFocus(cur, "b", "y"), cur);
  });

  it("同じマシンで shj のフォーカスが変われば追い、消えれば外す", () => {
    const cur = { target: "a", id: "x" };
    assert.deepEqual(adoptFocus(cur, "a", "y"), { target: "a", id: "y" });
    assert.equal(adoptFocus(cur, "a", null), null);
    assert.equal(adoptFocus(cur, "a", "x"), cur);
  });
});

describe("keepFocus", () => {
  it("フォーカス先が一覧から消えたら（切った・無くなった）外す", () => {
    const cur = { target: "a", id: "x" };
    assert.equal(keepFocus(cur, { a: [ws({ id: "x", root: "/x" })] }), cur);
    assert.equal(keepFocus(cur, { a: [] }), null);
    assert.equal(keepFocus(cur, {}), null);
    assert.equal(keepFocus(null, {}), null);
  });
});
