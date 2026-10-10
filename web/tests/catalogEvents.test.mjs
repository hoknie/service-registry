import assert from "node:assert/strict";
import { test } from "node:test";

import { catalogChanged, catalogVersion, onCatalogChange } from "../src/lib/catalogEvents.ts";

test("a change bumps the version and reaches every subscriber until it leaves", () => {
  const start = catalogVersion();
  const calls = [];
  const offA = onCatalogChange(() => calls.push("a"));
  const offB = onCatalogChange(() => calls.push("b"));
  catalogChanged();
  assert.equal(catalogVersion(), start + 1);
  assert.deepEqual(calls, ["a", "b"]);
  offA();
  catalogChanged();
  assert.deepEqual(calls, ["a", "b", "b"]);
  offB();
  catalogChanged();
  assert.equal(calls.length, 3);
  assert.equal(catalogVersion(), start + 3);
});
