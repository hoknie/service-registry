import assert from "node:assert/strict";
import { test } from "node:test";

import { nextRefreshMs, parseInterval, REFRESH_INTERVALS } from "../src/lib/autoRefresh.ts";

test("a stored interval is one of the offered values, anything else is off", () => {
  assert.deepEqual([...REFRESH_INTERVALS], [0, 10, 30, 60]);
  assert.equal(parseInterval("30"), 30);
  assert.equal(parseInterval("15"), 0);
  assert.equal(parseInterval(null), 0);
  assert.equal(parseInterval("abc"), 0);
});

test("the next refresh waits the interval and never runs while hidden or off", () => {
  assert.equal(nextRefreshMs(10, false), 10_000);
  assert.equal(nextRefreshMs(60, true), null);
  assert.equal(nextRefreshMs(0, false), null);
});
