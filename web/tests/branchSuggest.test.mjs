import assert from "node:assert/strict";
import { test } from "node:test";

import { branchSuggestions } from "../src/lib/branchSuggest.ts";

test("default branch comes first and is marked", () => {
  assert.deepEqual(branchSuggestions(["develop", "main", "release/1"], "", "main", "default"), [
    { value: "main", label: "main", detail: "default" },
    { value: "develop", label: "develop" },
    { value: "release/1", label: "release/1" },
  ]);
});

test("filters by name start", () => {
  assert.deepEqual(
    branchSuggestions(["main", "release/1", "Release/2", "feature/rel"], "rel", "main", "default").map((s) => s.value),
    ["release/1", "Release/2"],
  );
});

test("limit and missing default", () => {
  assert.equal(branchSuggestions(["a", "b", "c"], "", "x", "default", 2).length, 2);
  assert.deepEqual(branchSuggestions([], "m", null, "default"), []);
});
