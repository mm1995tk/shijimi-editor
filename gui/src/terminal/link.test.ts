import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { ErrorCode, RpcError } from "../rpc/message.ts";
import type { TermInfo } from "../rpc/protocol.ts";
import {
  decodeBase64,
  encodeBase64,
  EXITED_NOTICE,
  RECREATED_NOTICE,
  TermLink,
  type TermPort,
  type TermSink,
} from "./link.ts";

const enc = new TextEncoder();
const dec = new TextDecoder();
const b64 = (s: string) => encodeBase64(enc.encode(s));

/** 返事を手で返す偽の shj。 */
function fakePort(opts: { terms?: TermInfo[] } = {}) {
  const calls: { method: string; params: any }[] = [];
  const acks: { termId: string; seq: number }[] = [];
  const handlers: Record<string, (params: any) => unknown> = {
    "term.list": () => ({ terms: opts.terms ?? [] }),
    "term.create": () => ({ termId: "t1", pid: 1, cwd: "/w" }),
    "term.attach": () => ({ seq: 0, replay: "", cols: 80, rows: 24 }),
    "term.input": () => ({}),
    "term.resize": () => ({}),
    "term.close": () => ({}),
  };
  const port: TermPort = {
    async request(method, params) {
      calls.push({ method, params });
      const r = handlers[method]!(params);
      if (r instanceof Error) throw r;
      return r as never;
    },
    ackTerm(termId, seq) {
      acks.push({ termId, seq });
    },
  };
  return { port, calls, acks, handlers };
}

/** 書いたものを溜める偽の画面。done は手で呼ぶ（xterm.js の書き終わり）。 */
function fakeSink() {
  const out: string[] = [];
  const pending: (() => void)[] = [];
  let resets = 0;
  const sink: TermSink = {
    write(data, done) {
      out.push(dec.decode(data));
      pending.push(done);
    },
    notice(text) {
      out.push(text);
    },
    reset() {
      resets++;
      out.length = 0;
    },
  };
  return {
    sink,
    out,
    text: () => out.join(""),
    flush: () => pending.splice(0).forEach((f) => f()),
    resets: () => resets,
  };
}

/** term.list に出るターミナル。 */
function alive(termId: string, isAlive = true): TermInfo {
  return {
    termId,
    pid: 1,
    cwd: "/w",
    workspaceId: null,
    title: "",
    cols: 80,
    rows: 24,
    alive: isAlive,
  };
}

// 返事を待つ await がいくつか続くので、何回か回す
const tick = async () => {
  for (let i = 0; i < 10; i++) await new Promise((r) => setTimeout(r, 0));
};

function output(termId: string, seq: number, s: string) {
  return {
    method: "term.output" as const,
    params: { termId, seq, data: b64(s) },
  };
}

describe("TermLink", () => {
  it("作ったら 0 から取り直し、出力を書いて、書き終えた分を ack する", async () => {
    const f = fakePort();
    f.handlers["term.attach"] = () => ({
      seq: 2,
      replay: b64("$ "),
      cols: 80,
      rows: 24,
    });
    const s = fakeSink();
    const link = new TermLink(f.port);
    link.resize(100, 30);
    link.bind(s.sink);
    await tick();
    assert.deepEqual(
      f.calls.map((c) => c.method),
      ["term.list", "term.create", "term.attach"],
    );
    assert.deepEqual(f.calls[1]!.params, { cols: 100, rows: 30 });
    assert.deepEqual(f.calls[2]!.params, { termId: "t1", fromSeq: 0 });
    assert.equal(s.text(), "$ ");

    link.handle(output("t1", 7, "hello"));
    assert.equal(s.text(), "$ hello");
    assert.deepEqual(f.acks, []); // 書き終えるまで返さない
    s.flush();
    assert.deepEqual(f.acks, [{ termId: "t1", seq: 7 }]);
  });

  it("取り直した分と重なる出力は重なりを捨て、別のターミナルの出力は無視する", async () => {
    const f = fakePort();
    f.handlers["term.attach"] = () => ({
      seq: 4,
      replay: b64("abcd"),
      cols: 80,
      rows: 24,
    });
    const s = fakeSink();
    const link = new TermLink(f.port);
    link.bind(s.sink);
    await tick();
    link.handle(output("t1", 3, "abc")); // 全部取り直し済み
    link.handle(output("t1", 6, "cdef")); // cd は取り直し済み
    link.handle(output("t9", 10, "zzz"));
    assert.equal(s.text(), "abcdef");
  });

  it("seq が飛んだら、続きから term.attach で取り直す。その返事を待つ間の出力は捨てる", async () => {
    const f = fakePort();
    const s = fakeSink();
    const link = new TermLink(f.port);
    link.bind(s.sink);
    await tick();
    link.handle(output("t1", 3, "abc"));
    f.handlers["term.attach"] = () => ({
      seq: 20,
      replay: b64("[rest]"),
      cols: 80,
      rows: 24,
    });
    link.handle(output("t1", 15, "xyz")); // 3..12 が捨てられた
    link.handle(output("t1", 20, "late!")); // attach の返事待ちの間
    await tick();
    assert.deepEqual(f.calls.at(-1), {
      method: "term.attach",
      params: { termId: "t1", fromSeq: 3 },
    });
    assert.equal(s.text(), "abc[rest]");
    link.handle(output("t1", 22, "ok"));
    assert.equal(s.text(), "abc[rest]ok");
  });

  it("動いているターミナルがあれば、作らずにつないで大きさを合わせる", async () => {
    const f = fakePort({
      terms: [
        {
          termId: "t3",
          pid: 3,
          cwd: "/w",
          workspaceId: null,
          title: "",
          cols: 80,
          rows: 24,
          alive: true,
        },
      ],
    });
    const s = fakeSink();
    const link = new TermLink(f.port);
    link.resize(120, 40);
    link.bind(s.sink);
    await tick();
    assert.deepEqual(
      f.calls.map((c) => c.method),
      ["term.list", "term.attach", "term.resize"],
    );
    assert.deepEqual(f.calls[2]!.params, { termId: "t3", cols: 120, rows: 40 });
  });

  it("キー入力を base64 にして送り、大きさが変わったら term.resize を送る", async () => {
    const f = fakePort();
    const link = new TermLink(f.port);
    link.input(enc.encode("x")); // まだターミナルが無い
    link.bind(fakeSink().sink);
    await tick();
    link.input(enc.encode("ls\r"));
    link.resize(90, 20);
    link.resize(90, 20);
    await tick();
    assert.deepEqual(f.calls.slice(3), [
      { method: "term.input", params: { termId: "t1", data: b64("ls\r") } },
      { method: "term.resize", params: { termId: "t1", cols: 90, rows: 20 } },
    ]);
  });

  it("つなぎ直したら続きから取り直し、ターミナルが無ければ新しく作る", async () => {
    const f = fakePort();
    const s = fakeSink();
    const link = new TermLink(f.port);
    link.bind(s.sink);
    await tick();
    link.handle(output("t1", 2, "ab"));
    f.handlers["term.list"] = () => ({ terms: [alive("t1")] });
    f.calls.length = 0;
    link.reconnected();
    await tick();
    assert.deepEqual(f.calls, [
      { method: "term.attach", params: { termId: "t1", fromSeq: 2 } },
      { method: "term.list", params: {} },
    ]);

    // shj が作り直された
    f.handlers["term.list"] = () => ({ terms: [] });
    f.calls.length = 0;
    f.handlers["term.attach"] = (p) =>
      p.termId === "t1"
        ? new RpcError({
            code: ErrorCode.NotFound,
            message: "no such terminal",
          })
        : { seq: 0, replay: "", cols: 80, rows: 24 };
    f.handlers["term.create"] = () => ({ termId: "t2", pid: 2, cwd: "/w" });
    link.reconnected();
    await tick();
    assert.deepEqual(
      f.calls.map((c) => c.method),
      ["term.attach", "term.list", "term.create", "term.attach"],
    );
    assert.equal(link.id, "t2");
    assert.ok(s.text().endsWith(RECREATED_NOTICE));
  });

  it("シェルが終わったら残りを取ってから知らせ、Enter で新しいシェルを開く", async () => {
    const f = fakePort();
    const s = fakeSink();
    const link = new TermLink(f.port);
    link.bind(s.sink);
    await tick();
    f.handlers["term.attach"] = () => ({
      seq: 5,
      replay: b64("exit\n"),
      cols: 80,
      rows: 24,
    });
    link.handle({ method: "term.exit", params: { termId: "t1", code: 0 } });
    await tick();
    assert.equal(s.text(), "exit\n" + EXITED_NOTICE);

    f.calls.length = 0;
    link.input(enc.encode("a")); // Enter 以外は何もしない
    f.handlers["term.create"] = () => ({ termId: "t2", pid: 2, cwd: "/w" });
    f.handlers["term.attach"] = () => ({
      seq: 0,
      replay: "",
      cols: 80,
      rows: 24,
    });
    link.input(enc.encode("\r"));
    await tick();
    assert.deepEqual(
      f.calls.map((c) => c.method),
      ["term.close", "term.create", "term.attach"],
    );
    assert.equal(link.id, "t2");
    assert.equal(s.text(), "");
  });

  it("外したら出力を書かず ack もしない。また付けたら全部取り直す", async () => {
    const f = fakePort();
    const s = fakeSink();
    const link = new TermLink(f.port);
    link.bind(s.sink);
    await tick();
    link.handle(output("t1", 2, "ab"));
    link.unbind();
    s.flush(); // 外す前に書いた分の書き終わり
    link.handle(output("t1", 4, "cd"));
    assert.deepEqual(f.acks, []);

    const s2 = fakeSink();
    f.handlers["term.list"] = () => ({ terms: [alive("t1")] });
    f.calls.length = 0;
    link.bind(s2.sink);
    await tick();
    assert.deepEqual(f.calls, [
      { method: "term.attach", params: { termId: "t1", fromSeq: 0 } },
      { method: "term.list", params: {} },
    ]);
  });

  it("取り直した分を画面が書き終える前に欠けが続いても、取り直しは書き終えてから 1 回だけ", async () => {
    const f = fakePort();
    const s = fakeSink();
    const link = new TermLink(f.port);
    link.bind(s.sink);
    await tick();
    s.flush();
    let attaches = 0;
    f.handlers["term.attach"] = () => {
      attaches++;
      return { seq: attaches * 100, replay: b64("R"), cols: 80, rows: 24 };
    };
    link.handle(output("t1", 10, "x")); // 0..9 が欠けた
    await tick();
    assert.equal(attaches, 1);
    // 書き終える前に、また欠ける
    link.handle(output("t1", 150, "y"));
    link.handle(output("t1", 180, "z"));
    await tick();
    assert.equal(attaches, 1);
    s.flush(); // 取り直した分を書き終えた
    await tick();
    assert.equal(attaches, 2);
    assert.deepEqual(f.calls.at(-1)!.params, { termId: "t1", fromSeq: 100 });
    s.flush();
    await tick();
    assert.equal(attaches, 2); // 欠けが無ければもう取り直さない
  });

  it("取り直しの返事を待つ間のキー入力も送る", async () => {
    const f = fakePort();
    const link = new TermLink(f.port);
    link.bind(fakeSink().sink);
    await tick();
    let answer!: () => void;
    f.handlers["term.attach"] = () =>
      new Promise((r) => {
        answer = () => r({ seq: 20, replay: "", cols: 80, rows: 24 });
      });
    link.handle(output("t1", 20, "x")); // 欠けたので取り直す
    link.input(enc.encode("\x03"));
    await tick();
    assert.deepEqual(f.calls.at(-1), {
      method: "term.input",
      params: { termId: "t1", data: b64("\x03") },
    });
    answer();
  });

  it("外している間に終わったシェルは、付け直したときに知らせて Enter で作り直せる", async () => {
    const f = fakePort();
    const link = new TermLink(f.port);
    link.bind(fakeSink().sink);
    await tick();
    link.unbind();
    link.handle({ method: "term.exit", params: { termId: "t1", code: 0 } }); // 外しているので届かない
    f.handlers["term.list"] = () => ({ terms: [alive("t1", false)] });
    const s = fakeSink();
    link.bind(s.sink);
    await tick();
    assert.ok(s.text().endsWith(EXITED_NOTICE));
    assert.equal(s.text().split(EXITED_NOTICE).length - 1, 1); // 1 回だけ
    f.calls.length = 0;
    f.handlers["term.create"] = () => ({ termId: "t2", pid: 2, cwd: "/w" });
    link.input(enc.encode("\r"));
    await tick();
    assert.deepEqual(
      f.calls.map((c) => c.method),
      ["term.close", "term.create", "term.attach"],
    );
  });

  it("term.input が not_found なら、シェルが終わったものとして知らせる", async () => {
    const f = fakePort();
    const s = fakeSink();
    const link = new TermLink(f.port);
    link.bind(s.sink);
    await tick();
    f.handlers["term.input"] = () =>
      new RpcError({
        code: ErrorCode.NotFound,
        message: "terminal has exited",
      });
    link.input(enc.encode("a"));
    await tick();
    assert.ok(s.text().endsWith(EXITED_NOTICE));
  });
  it("ワークスペースを移したら、そのワークスペースのターミナルに切り替えて取り直し、大きさを合わせる", async () => {
    const f = fakePort({ terms: [alive("t1"), { ...alive("t2"), cols: 50 }] });
    const s = fakeSink();
    const link = new TermLink(f.port);
    link.resize(100, 30);
    link.bind(s.sink);
    await tick();
    link.handle(output("t1", 3, "one"));
    f.handlers["term.attach"] = (p) => ({
      seq: 3,
      replay: b64(p.termId === "t2" ? "two" : ""),
      cols: 50,
      rows: 24,
    });
    f.calls.length = 0;
    link.switchTo("t2", "/b");
    await tick();
    assert.equal(link.id, "t2");
    assert.equal(s.text(), "two", "前のターミナルの中身を消してから描く");
    assert.deepEqual(f.calls[0], {
      method: "term.attach",
      params: { termId: "t2", fromSeq: 0 },
    });
    assert.ok(
      f.calls.some(
        (c) =>
          c.method === "term.resize" &&
          c.params.termId === "t2" &&
          c.params.cols === 100,
      ),
    );
    link.handle(output("t1", 6, "xxx")); // 前のターミナルの出力は書かない
    assert.equal(s.text(), "two");
    assert.ok(
      !f.calls.some((c) => c.method === "term.close"),
      "前のシェルは止めない",
    );

    // 同じターミナルなら何もしない
    f.calls.length = 0;
    link.switchTo("t2", "/b");
    await tick();
    assert.deepEqual(f.calls, []);
  });

  it("ワークスペースにターミナルが無ければ、ほかのシェルを拾わずにルートで作る", async () => {
    const f = fakePort({ terms: [alive("t1")] });
    f.handlers["term.create"] = () => ({ termId: "t9", pid: 2, cwd: "/b" });
    const s = fakeSink();
    const link = new TermLink(f.port);
    link.bind(s.sink);
    await tick();
    assert.equal(link.id, "t1");
    f.calls.length = 0;
    link.switchTo(undefined, "/b");
    await tick();
    assert.equal(link.id, "t9");
    assert.deepEqual(
      f.calls.map((c) => c.method),
      ["term.create", "term.attach"],
    );
    assert.equal(f.calls[0]!.params.cwd, "/b");
  });

  it("外している間に移したら覚えておき、付けたときにそのターミナルを描く", async () => {
    const f = fakePort({ terms: [alive("t1"), alive("t2")] });
    const s = fakeSink();
    const link = new TermLink(f.port);
    link.switchTo("t2", "/b");
    await tick();
    assert.deepEqual(f.calls, [], "外している間は何も送らない");
    link.bind(s.sink);
    await tick();
    assert.equal(link.id, "t2");
    assert.deepEqual(f.calls[0], {
      method: "term.attach",
      params: { termId: "t2", fromSeq: 0 },
    });
  });

  it("移したあとにシェルが終わったら、Enter でそのワークスペースのルートに作り直す", async () => {
    const f = fakePort({ terms: [alive("t2")] });
    f.handlers["term.create"] = () => ({ termId: "t3", pid: 3, cwd: "/b" });
    const s = fakeSink();
    const link = new TermLink(f.port);
    link.switchTo("t2", "/b");
    link.bind(s.sink);
    await tick();
    link.handle({ method: "term.exit", params: { termId: "t2", code: 0 } });
    await tick();
    f.calls.length = 0;
    link.input(enc.encode("\r"));
    await tick();
    const create = f.calls.find((c) => c.method === "term.create");
    assert.equal(create?.params.cwd, "/b");
    assert.equal(link.id, "t3");
  });
});

describe("base64", () => {
  it("どのバイトも行き来できる", () => {
    const all = Uint8Array.from({ length: 256 }, (_, i) => i);
    assert.deepEqual(decodeBase64(encodeBase64(all)), all);
    const big = new Uint8Array(100_000).fill(0x79);
    assert.equal(decodeBase64(encodeBase64(big)).length, 100_000);
  });
});
