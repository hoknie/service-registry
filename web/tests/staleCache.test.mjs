import assert from "node:assert/strict";
import { test } from "node:test";

import { staleCache } from "../src/lib/staleCache.ts";

const settle = () => new Promise((r) => setTimeout(r, 5));

test("a stale entry is reloaded when read and keeps its value on failure", async () => {
  let emitted = 0;
  const cache = staleCache(() => emitted++, (e) => String(e));
  let answer = () => Promise.resolve(["a"]);
  let calls = 0;
  const load = () => {
    calls++;
    return answer();
  };

  cache.ensure("rsqa", load);
  await settle();
  assert.deepEqual(cache.read("rsqa").value, ["a"]);
  cache.ensure("rsqa", load);
  await settle();
  assert.equal(calls, 1, "a loaded entry is not fetched again");

  answer = () => Promise.resolve(["a", "b"]);
  cache.revalidate();
  assert.equal(calls, 1, "revalidate itself sends nothing");
  assert.deepEqual(cache.read("rsqa").value, ["a"], "the old value stays while reloading");
  await settle();
  assert.equal(calls, 2);
  assert.deepEqual(cache.read("rsqa").value, ["a", "b"]);

  answer = () => Promise.reject(new Error("boom"));
  cache.revalidate();
  cache.read("rsqa");
  await settle();
  assert.equal(calls, 3);
  assert.deepEqual(cache.get("rsqa").value, ["a", "b"]);
  assert.equal(cache.get("rsqa").error, undefined);
  assert.ok(emitted > 0);
});

test("unread stale entries wait until they are read or ensured", async () => {
  const cache = staleCache(() => undefined, String);
  let calls = 0;
  const load = () => Promise.resolve(++calls);
  cache.ensure("x", load);
  await settle();
  cache.revalidate();
  await settle();
  assert.equal(calls, 1);
  cache.ensure("x", load);
  await settle();
  assert.equal(calls, 2);
  assert.equal(cache.get("x").value, 2);
});
