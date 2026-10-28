import assert from "node:assert/strict";
import { test } from "node:test";

import { BUSY_MS, IDLE_MS, busy, pollDelay, settled, visible } from "../src/lib/activity.ts";

const p = (kind, state) => ({ kind, state, code: null, last_at: null, pending: null });

test("polls fast while something runs or waits", () => {
  assert.equal(pollDelay({ processes: [p("collect", "running")] }), BUSY_MS);
  assert.equal(pollDelay({ processes: [p("repo_sync", "queued")] }), BUSY_MS);
  assert.equal(pollDelay({ summary: { running: 0, queued: 2, failed: 0 } }), BUSY_MS);
  assert.equal(pollDelay({ processes: [p("collect", "failed"), p("index", "idle")] }), IDLE_MS);
  assert.equal(pollDelay({ summary: { running: 0, queued: 0, failed: 3 } }), IDLE_MS);
  assert.equal(pollDelay(null), IDLE_MS);
  assert.equal(busy([p("index", "running")]), true);
});

test("reports processes that stopped running", () => {
  const before = { processes: [p("collect", "running"), p("index", "running"), p("repo_sync", "queued")] };
  assert.deepEqual(settled(before, { processes: [p("collect", "idle"), p("index", "running"), p("repo_sync", "running")] }), ["collect"]);
  assert.deepEqual(settled(before, { processes: [p("index", "failed")] }), ["index", "collect"]);
  assert.deepEqual(settled(null, before), []);
});

test("reports a container whose subtree stopped running and its own repository sync run", () => {
  const sum = (running) => ({ running, queued: 0, failed: 0 });
  assert.deepEqual(settled({ summary: sum(2), processes: [] }, { summary: sum(0), processes: [] }), ["subtree"]);
  assert.deepEqual(settled({ summary: sum(0), processes: [p("repo_sync", "running")] }, { summary: sum(0), processes: [p("repo_sync", "idle")] }), ["repo_sync"]);
  assert.deepEqual(settled({ summary: sum(1) }, { summary: sum(1) }), []);
  assert.equal(busy({ summary: sum(0), processes: [p("repo_sync", "queued")] }), true);
});

test("hides idle processes", () => {
  assert.deepEqual(visible([p("collect", "idle"), p("repo_sync", "failed")]).map((x) => x.kind), ["repo_sync"]);
});
