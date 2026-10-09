import assert from "node:assert/strict";
import { test } from "node:test";

import { filterOptions, splitHighlight } from "../src/lib/combobox.ts";

const options = [
  { value: "1", label: "acme / backend", keywords: ["backend"] },
  { value: "2", label: "acme / frontend" },
  { value: "3", label: "Billing", detail: "billing@example.com" },
];

test("an empty query keeps every option without a highlight", () => {
  const all = filterOptions(options, "  ");
  assert.equal(all.length, 3);
  assert.equal(all[0].start, -1);
});

test("a query matches the label by substring, case-insensitively, with its range", () => {
  const m = filterOptions(options, "BACK");
  assert.deepEqual(m.map((x) => [x.option.value, x.start, x.end]), [["1", 7, 11]]);
  assert.deepEqual(splitHighlight(m[0].option.label, m[0].start, m[0].end), ["acme / ", "back", "end"]);
});

test("details and keywords match without a highlight; nothing found is empty", () => {
  assert.deepEqual(filterOptions(options, "example.com").map((x) => [x.option.value, x.start]), [["3", -1]]);
  assert.equal(filterOptions(options, "zzz").length, 0);
  assert.equal(filterOptions(options, "", 2).length, 2);
});
