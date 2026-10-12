import assert from "node:assert/strict";
import { test } from "node:test";

import { alertKind, headingSlug, taskMarker, uniqueSlug } from "../src/lib/readme.ts";

test("heading slugs keep letters of any script and drop punctuation", () => {
  assert.equal(headingSlug("Требования к API"), "требования-к-api");
  assert.equal(headingSlug("  Hello, World!  "), "hello-world");
  assert.equal(headingSlug("v1.2 — notes"), "v12-notes");
  assert.equal(headingSlug("服务 列表"), "服务-列表");
});

test("repeated slugs get a numeric suffix", () => {
  const used = new Map();
  assert.deepEqual(["a", "a", "a", "a-1"].map((s) => uniqueSlug(s, used)), ["a", "a-1", "a-2", "a-1-1"]);
});

test("task markers are recognized only at the start with a space after", () => {
  assert.deepEqual(taskMarker("[ ] todo"), { checked: false, rest: "todo" });
  assert.deepEqual(taskMarker("[x] done"), { checked: true, rest: "done" });
  assert.deepEqual(taskMarker("[X] Done"), { checked: true, rest: "Done" });
  assert.equal(taskMarker("[x]done"), null);
  assert.equal(taskMarker("see [x] here"), null);
  assert.equal(taskMarker("[-] no"), null);
});

test("alert markers are the five GitHub kinds", () => {
  assert.equal(alertKind("[!NOTE]"), "note");
  assert.equal(alertKind("[!WARNING]  "), "warning");
  assert.equal(alertKind("[!note]"), null);
  assert.equal(alertKind("[!DANGER]"), null);
  assert.equal(alertKind("[!TIP] extra"), null);
});
