import assert from "node:assert/strict";
import { test } from "node:test";

import { BUSY_MS, IDLE_MS, busy, pollDelay, settled, visible } from "../src/lib/activity.ts";

const p = (kind, state) => ({ kind, state, code: null, last_at: null, pending: null });

test("polls fast while something runs or waits", () => {
  assert.equal(pollDelay({ processes: [p("collect", "running")] }), BUSY_MS);
  assert.equal(pollDelay({ processes: [p("forge", "queued")] }), BUSY_MS);
  assert.equal(pollDelay({ summary: { running: 0, queued: 2, failed: 0 } }), BUSY_MS);
  assert.equal(pollDelay({ processes: [p("collect", "failed"), p("index", "idle")] }), IDLE_MS);
  assert.equal(pollDelay({ summary: { running: 0, queued: 0, failed: 3 } }), IDLE_MS);
  assert.equal(pollDelay(null), IDLE_MS);
  assert.equal(busy([p("index", "running")]), true);
});

test("reports processes that stopped running", () => {
  const before = { processes: [p("collect", "running"), p("index", "running"), p("forge", "queued")] };
  assert.deepEqual(settled(before, { processes: [p("collect", "idle"), p("index", "running"), p("forge", "running")] }), ["collect"]);
  assert.deepEqual(settled(before, { processes: [p("index", "failed")] }), ["index", "collect"]);
  assert.deepEqual(settled(null, before), []);
});

test("hides idle processes", () => {
  assert.deepEqual(visible([p("collect", "idle"), p("forge", "failed")]).map((x) => x.kind), ["forge"]);
});
