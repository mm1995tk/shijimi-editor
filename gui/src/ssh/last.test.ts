import assert from "node:assert/strict";
import { test } from "node:test";

import {
  autoConnectTargets,
  LAST_KEY,
  loadLastTargets,
  recordWorkspaces,
  type KeyValueStore,
} from "./last.ts";

function mem(init?: string): KeyValueStore & { m: Map<string, string> } {
  const m = new Map<string, string>();
  if (init !== undefined) m.set(LAST_KEY, init);
  return {
    m,
    getItem: (k) => m.get(k) ?? null,
    setItem: (k, v) => void m.set(k, v),
  };
}

test("記録が無い・壊れている・store が無いときは空", () => {
  assert.deepEqual(loadLastTargets(mem()), []);
  assert.deepEqual(loadLastTargets(mem("{")), []);
  assert.deepEqual(loadLastTargets(mem('{"a":1}')), []);
  assert.deepEqual(loadLastTargets(undefined), []);
  assert.deepEqual(loadLastTargets(mem('["a",1,"b"]')), ["a", "b"]);
});

test("ワークスペースがあれば加え、無くなれば外す", () => {
  const s = mem();
  recordWorkspaces(s, "devbox", 2);
  recordWorkspaces(s, "@local", 1);
  recordWorkspaces(s, "devbox", 3); // 重ねて入れない
  assert.deepEqual(loadLastTargets(s), ["devbox", "@local"]);
  recordWorkspaces(s, "devbox", 0);
  assert.deepEqual(loadLastTargets(s), ["@local"]);
  recordWorkspaces(s, "nowhere", 0); // 無いものを外しても何も起きない
  assert.deepEqual(loadLastTargets(s), ["@local"]);
});

test("書けない store でも落ちない", () => {
  const s: KeyValueStore = {
    getItem: () => null,
    setItem: () => {
      throw new Error("quota");
    },
  };
  recordWorkspaces(s, "a", 1);
});

test("自動でつなぐのは、記録にあって選べるものだけ", () => {
  assert.deepEqual(
    autoConnectTargets(["a", "gone", "@local"], ["@local", "a", "b"]),
    ["a", "@local"],
  );
});
