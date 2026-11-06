import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { CopiedNotice } from "./copiedNotice.ts";

describe("CopiedNotice", () => {
  it("出して、時間が経つと自分で消える", async () => {
    const log: boolean[] = [];
    const n = new CopiedNotice((v) => log.push(v), 10);
    n.show();
    assert.deepEqual(log, [true]);
    await new Promise((r) => setTimeout(r, 40));
    assert.deepEqual(log, [true, false]);
  });

  it("出ている間にもう一度出すと、数え直す", async () => {
    const log: boolean[] = [];
    const n = new CopiedNotice((v) => log.push(v), 30);
    n.show();
    await new Promise((r) => setTimeout(r, 20));
    n.show();
    await new Promise((r) => setTimeout(r, 20));
    assert.deepEqual(log, [true, true]);
    await new Promise((r) => setTimeout(r, 40));
    assert.deepEqual(log, [true, true, false]);
  });
});
