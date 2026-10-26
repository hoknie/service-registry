import assert from "node:assert/strict";
import { test } from "node:test";

import { addTable, MAX_TABLES, readTables, removeTable, updateTable } from "../src/lib/metricTables.ts";

const ids = () => {
  let n = 0;
  return () => `n${++n}`;
};

test("a missing or broken store gives one default table", () => {
  assert.deepEqual(readTables(null, ids()), [{ id: "n1", metric: "", filter: "" }]);
  assert.deepEqual(readTables("{oops", ids()), [{ id: "n1", metric: "", filter: "" }]);
  assert.deepEqual(readTables('{"a":1}', ids()), [{ id: "n1", metric: "", filter: "" }]);
});

test("a stored set keeps order, fixes bad items and is capped", () => {
  const raw = JSON.stringify([{ id: "a", metric: "go_goroutines", filter: "x", extra: 1 }, { id: "a", metric: 5 }, null, { metric: "m" }]);
  assert.deepEqual(readTables(raw, ids()), [
    { id: "a", metric: "go_goroutines", filter: "x" },
    { id: "n1", metric: "", filter: "" },
    { id: "n2", metric: "m", filter: "" },
  ]);
  assert.deepEqual(readTables("[]", ids()), []);
  const many = JSON.stringify(Array.from({ length: 15 }, (_, i) => ({ id: `t${i}`, metric: "m", filter: "" })));
  assert.equal(readTables(many, ids()).length, MAX_TABLES);
});

test("tables are added up to the limit, updated and removed", () => {
  let t = [];
  for (let i = 0; i < 12; i++) t = addTable(t, `t${i}`, "m");
  assert.equal(t.length, MAX_TABLES);
  t = updateTable(t, "t1", { metric: "x", filter: "f" });
  assert.deepEqual(t[1], { id: "t1", metric: "x", filter: "f" });
  t = removeTable(t, "t0");
  assert.equal(t[0].id, "t1");
  assert.equal(t.length, MAX_TABLES - 1);
});
