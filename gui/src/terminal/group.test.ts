import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { TermInfo } from "../rpc/protocol.ts";
import { TermGroup } from "./group.ts";
import { EXITED_NOTICE, type TermPort, type TermSink } from "./link.ts";

/** 返事をすぐ返す偽の shj。term.create は t1, t2, … と作る。 */
function fakePort(terms: TermInfo[] = []) {
  const calls: { method: string; params: any }[] = [];
  let next = 0;
  const handlers: Record<string, (params: any) => unknown> = {
    "term.list": () => ({ terms }),
    "term.create": (p) => ({
      termId: `t${++next}`,
      pid: 1,
      cwd: p.cwd ?? "~",
    }),
    "term.attach": () => ({ seq: 0, replay: "", cols: 80, rows: 24 }),
    "term.input": () => ({}),
    "term.resize": () => ({}),
    "term.close": () => ({}),
  };
  const port: TermPort = {
    async request(method, params) {
      calls.push({ method, params });
      return handlers[method]!(params) as never;
    },
    ackTerm() {},
  };
  return { port, calls, handlers };
}

function sink(): TermSink & { text: string } {
  const s = {
    text: "",
    write(data: Uint8Array, done: () => void) {
      s.text += new TextDecoder().decode(data);
      done();
    },
    notice(t: string) {
      s.text += t;
    },
    reset() {
      s.text = "";
    },
  };
  return s;
}

function info(termId: string, cwd = "/w", alive = true): TermInfo {
  return {
    termId,
    pid: 1,
    cwd,
    workspaceId: null,
    title: "",
    cols: 80,
    rows: 24,
    alive,
  };
}

const tick = async () => {
  for (let i = 0; i < 10; i++) await new Promise((r) => setTimeout(r, 0));
};

/** 画面の代わりに、並んだターミナルを全部 bind する。 */
function bindAll(g: TermGroup, sinks = new Map<unknown, TermSink>()) {
  for (const l of g.list) {
    if (sinks.has(l)) continue;
    const s = sink();
    sinks.set(l, s);
    l.bind(s);
  }
  return sinks;
}

describe("TermGroup", () => {
  it("出したら動いているシェルを全部拾って並べ、終わったものは拾わない", async () => {
    const f = fakePort([info("a"), info("b", "/x", false), info("c")]);
    let changes = 0;
    const g = new TermGroup(f.port, () => changes++);
    g.show();
    await tick();
    bindAll(g);
    await tick();
    assert.deepEqual(
      g.list.map((l) => l.id),
      ["a", "c"],
    );
    assert.equal(g.active, g.list[0]);
    assert.ok(changes > 0);
    assert.ok(!f.calls.some((c) => c.method === "term.create"));
  });

  it("出す前は何もしない。動いているシェルが無ければ 1 つ作る", async () => {
    const f = fakePort();
    const g = new TermGroup(f.port, () => {});
    await tick();
    assert.equal(f.calls.length, 0);
    g.show();
    await tick();
    bindAll(g);
    await tick();
    assert.deepEqual(
      g.list.map((l) => l.id),
      ["t1"],
    );
    // 作る欄は、ほかのシェルを拾いに term.list をもう一度引かない
    assert.deepEqual(
      f.calls.map((c) => c.method),
      ["term.list", "term.create", "term.attach"],
    );
  });

  it("足すと active になり、cwd を付けて作る。6 個までしか足せない", async () => {
    const f = fakePort();
    const g = new TermGroup(f.port, () => {});
    g.show();
    await tick();
    const sinks = bindAll(g);
    await tick();
    assert.equal(g.add("/repo"), true);
    bindAll(g, sinks);
    await tick();
    assert.equal(g.active, g.list[1]);
    assert.equal(g.active?.id, "t2");
    const create = f.calls.filter((c) => c.method === "term.create");
    assert.equal(create[1]!.params.cwd, "/repo");
    for (let i = 0; i < 4; i++) assert.equal(g.add(), true);
    assert.equal(g.list.length, 6);
    assert.equal(g.add(), false);
    assert.equal(g.list.length, 6);
  });

  it("矢印で隣へ移り、端では動かない", async () => {
    const f = fakePort([info("a"), info("b"), info("c"), info("d")]);
    const g = new TermGroup(f.port, () => {});
    g.show();
    await tick();
    assert.equal(g.move("right"), true);
    assert.equal(g.active?.id, "b");
    assert.equal(g.move("down"), true);
    assert.equal(g.active?.id, "d");
    assert.equal(g.move("right"), false);
    assert.equal(g.active?.id, "d");
    g.setActive(2);
    assert.equal(g.active?.id, "c");
  });

  it("シェルが終わったら欄を閉じて並び直し、隣を active にする。最後の 1 つは残す", async () => {
    const f = fakePort([info("a"), info("b"), info("c")]);
    const g = new TermGroup(f.port, () => {});
    g.show();
    await tick();
    const sinks = bindAll(g);
    await tick();
    g.setActive(1);
    g.handle({ method: "term.exit", params: { termId: "b", code: 0 } });
    await tick();
    assert.deepEqual(
      g.list.map((l) => l.id),
      ["a", "c"],
    );
    assert.equal(g.active?.id, "c");
    assert.ok(
      f.calls.some((c) => c.method === "term.close" && c.params.termId === "b"),
    );

    g.handle({ method: "term.exit", params: { termId: "c", code: 0 } });
    await tick();
    g.handle({ method: "term.exit", params: { termId: "a", code: 0 } });
    await tick();
    assert.deepEqual(
      g.list.map((l) => l.id),
      ["a"],
    );
    const last = sinks.get(g.list[0]) as TermSink & { text: string };
    assert.ok(last.text.endsWith(EXITED_NOTICE));

    // 終わったまま残った欄は、新しく足すときに閉じる
    g.add();
    assert.equal(g.list.length, 1);
    assert.notEqual(g.list[0]?.id, "a");
  });

  it("ワークスペースを移したら、そのシェルを active にし、並べていなければ足し、無ければルートで作る", async () => {
    const f = fakePort([info("a"), info("b")]);
    const g = new TermGroup(f.port, () => {});
    g.show();
    await tick();
    const sinks = bindAll(g);
    await tick();
    g.switchTo("b", "/b");
    assert.equal(g.active?.id, "b");
    assert.equal(g.list.length, 2);

    // ほかの GUI が作ったものなど（生きているかを確かめる term.list には出る）
    f.handlers["term.list"] = () => ({
      terms: [info("a"), info("b"), info("z")],
    });
    g.switchTo("z", "/z");
    assert.equal(g.active?.id, "z");
    assert.equal(g.list.length, 3);

    g.switchTo(undefined, "/new");
    bindAll(g, sinks);
    await tick();
    assert.equal(g.list.length, 4);
    const create = f.calls.filter((c) => c.method === "term.create");
    assert.equal(create.at(-1)!.params.cwd, "/new");
    assert.equal(g.active, g.list[3]);
  });

  it("出す前に移したシェルも、出したときに拾うシェルと重ねずに並べる", async () => {
    const f = fakePort([info("a"), info("b")]);
    const g = new TermGroup(f.port, () => {});
    g.switchTo("b", "/b");
    g.show();
    await tick();
    assert.deepEqual(
      g.list.map((l) => l.id),
      ["a", "b"],
    );
    assert.equal(g.active?.id, "b");
  });
});
