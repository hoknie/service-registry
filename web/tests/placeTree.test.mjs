import assert from "node:assert/strict";
import { test } from "node:test";

import { countText, loadLevel, parentPath } from "../src/lib/placeTree.ts";

test("the first page of a level is its folders and then its items", async () => {
  const calls = [];
  const items = async (cursor) => {
    calls.push(cursor);
    return cursor === null ? { items: ["a", "b"], next_cursor: "c1" } : { items: ["c"], next_cursor: null };
  };
  const first = await loadLevel(null, async () => ["f1", "f2"], items);
  assert.deepEqual(first, { items: ["f1", "f2", "a", "b"], next: "c1" });
  const second = await loadLevel("c1", async () => assert.fail("folders are read once"), items);
  assert.deepEqual(second, { items: ["c"], next: null });
  assert.deepEqual(calls, [null, "c1"]);
});

test("a count read by cursor says when there is more", () => {
  assert.equal(countText(12, true), "12+");
  assert.equal(countText(3, false), "3");
});

test("the folder of a skill is its path without the last segment", () => {
  assert.equal(parentPath("go/testing/go-race"), "go/testing");
  assert.equal(parentPath("top-level"), "");
});
