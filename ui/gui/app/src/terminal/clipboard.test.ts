import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { pickReadMime, planWrite, supportsLocation } from "./clipboard.ts";

const enc = (s: string) => new TextEncoder().encode(s);

describe("planWrite", () => {
  it("text/plain はテキストとして書く", () => {
    assert.deepEqual(planWrite([{ mime: "text/plain", data: enc("héllo") }]), {
      kind: "text",
      text: "héllo",
    });
  });
  it("テキストと画像の両方があればテキストを選ぶ", () => {
    const plan = planWrite([
      { mime: "image/png", data: new Uint8Array([1]) },
      { mime: "text/plain;charset=utf-8", data: enc("a") },
    ]);
    assert.equal(plan?.kind, "text");
  });
  it("画像だけなら画像", () => {
    const bytes = new Uint8Array([1, 2]);
    assert.deepEqual(planWrite([{ mime: "image/png", data: bytes }]), {
      kind: "image",
      bytes,
    });
  });
  it("書けない種類だけなら undefined", () => {
    assert.equal(
      planWrite([{ mime: "text/html", data: enc("<b>x</b>") }]),
      undefined,
    );
  });
});

describe("pickReadMime", () => {
  it("先に書いた種類を優先する", () => {
    assert.equal(
      pickReadMime(["image/png", "text/plain"], ["text/plain", "image/png"]),
      "image/png",
    );
  });
  it("渡せない種類は飛ばす", () => {
    assert.equal(
      pickReadMime(["text/html", "text/plain"], ["text/plain"]),
      "text/plain",
    );
  });
  it("渡せるものが無ければ undefined", () => {
    assert.equal(pickReadMime(["image/png"], ["text/plain"]), undefined);
  });
});

describe("supportsLocation", () => {
  it("primary は対応しない", () => {
    assert.equal(supportsLocation("clipboard"), true);
    assert.equal(supportsLocation("primary"), false);
  });
});
