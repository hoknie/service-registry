import assert from "node:assert/strict";
import { test } from "node:test";

import { labelQuery, labelsPath, labelSuggestions, lastSegment, pickFilter, pickLabel } from "../src/lib/labelSuggest.ts";

test("text before = asks for keys, after = for values of that key", () => {
  assert.deepEqual(labelQuery(" te"), { key: null, q: "te" });
  assert.deepEqual(labelQuery("team=pay"), { key: "team", q: "pay" });
  assert.deepEqual(labelQuery("team="), { key: "team", q: "" });
  assert.equal(labelsPath({ key: null, q: "t&x" }), "/v1/catalog/labels?q=t%26x");
  assert.equal(labelsPath({ key: "team", q: "" }), "/v1/catalog/labels?key=team&q=");
});

test("a key continues the draft, a value commits the label", () => {
  assert.deepEqual(pickLabel("te", { value: "team", label: "team" }), { draft: "team=", commit: false });
  assert.deepEqual(pickLabel("team=p", { value: "payments", label: "team=payments" }), { draft: "team=payments", commit: true });
  assert.deepEqual(pickLabel("flag=", { value: "", label: "flag" }), { draft: "flag", commit: true });
});

test("suggestions show counts; an empty value shows the bare key", () => {
  assert.deepEqual(labelSuggestions({ key: null, q: "" }, [{ key: "team", count: 3 }], null), [{ value: "team", label: "team", detail: "3" }]);
  assert.deepEqual(
    labelSuggestions({ key: "flag", q: "" }, null, [
      { value: "", count: 1 },
      { value: "on", count: 2 },
    ]),
    [
      { value: "", label: "flag", detail: "1" },
      { value: "on", label: "flag=on", detail: "2" },
    ],
  );
});

test("the tree filter completes only its last comma-separated label", () => {
  assert.deepEqual(lastSegment("tier=1, te"), { head: "tier=1, ", tail: "te" });
  assert.deepEqual(lastSegment("te"), { head: "", tail: "te" });
  assert.equal(pickFilter("tier=1, te", { value: "team", label: "team" }), "tier=1, team=");
  assert.equal(pickFilter("tier=1,team=p", { value: "payments", label: "team=payments" }), "tier=1, team=payments");
});
