import assert from "node:assert/strict";
import { test } from "node:test";

import { addExample, duplicateRows, invalidRows, refQuery, refsOf, splitVersion, versionFromParams, versionKey, withKind } from "../src/lib/refs.ts";

test("version keys keep tags apart from branches", () => {
  assert.equal(versionKey("branch", "main"), "main");
  assert.equal(versionKey("tag", "v1.0.0"), "tag:v1.0.0");
  assert.deepEqual(splitVersion("tag:v1.0.0"), { kind: "tag", name: "v1.0.0" });
  assert.deepEqual(splitVersion("feature/x"), { kind: "branch", name: "feature/x" });
  assert.equal(refQuery("tag:v1", { path: "README.md" }).toString(), "path=README.md&tag=v1");
  assert.equal(refQuery("main").toString(), "branch=main");
  assert.equal(refQuery(null).toString(), "");
});

test("the tag parameter wins in an address", () => {
  assert.equal(versionFromParams(new URLSearchParams("branch=main&tag=v2")), "tag:v2");
  assert.equal(versionFromParams(new URLSearchParams("branch=dev")), "dev");
  assert.equal(versionFromParams(new URLSearchParams("branch=")), null);
});

test("rows of the refs field", () => {
  const tag = { pattern: "v*", kind: "tag", release: true };
  assert.deepEqual(withKind(tag, "branch"), { pattern: "v*", kind: "branch", release: false });
  assert.deepEqual(withKind({ ...tag, release: false }, "tag"), { pattern: "v*", kind: "tag", release: false });
  const rows = [
    { pattern: "feat/*", kind: "branch", release: false },
    { pattern: " feat/* ", kind: "branch", release: false },
    { pattern: "feat/*", kind: "tag", release: false },
    { pattern: "", kind: "tag", release: false },
  ];
  assert.deepEqual([...duplicateRows(rows)], [1]);
  assert.deepEqual(refsOf(rows).map((r) => r.pattern), ["feat/*", "feat/*", "feat/*"]);
  const valid = (p) => !p.includes("[");
  assert.equal(invalidRows(rows, valid), true);
  assert.equal(invalidRows([rows[0], { pattern: "v[", kind: "tag", release: false }], valid), true);
  assert.equal(invalidRows([rows[0], rows[2]], valid), false);
  assert.equal(addExample([rows[0]], rows[0]).length, 1);
  assert.equal(addExample([rows[0]], tag).length, 2);
});
