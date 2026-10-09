import assert from "node:assert/strict";
import { test } from "node:test";

import {
  buildRows,
  filtersQuery,
  groupByParent,
  hasFilters,
  highlight,
  openQuery,
  parseFilters,
  parseOpen,
  toggleOpen,
} from "../src/lib/treeTable.ts";

const row = (id, parent, children = 0, extra = {}) => ({ id, parent_id: parent, kind: "folder", slug: id, name: id, access: "read", children, match: true, activity: [], ...extra });
const branch = (rows, total = rows.length, more = {}) => ({ rows, total, loading: false, error: null, ...more });

test("filters round-trip through the address", () => {
  const f = parseFilters(new URLSearchParams("q=bill&kind=project&label=team=payments,tier&activity=failed"));
  assert.deepEqual(f, { q: "bill", kind: "project", label: "team=payments,tier", activity: "failed" });
  assert.equal(filtersQuery(f), "q=bill&kind=project&label=team%3Dpayments&label=tier&activity=failed");
  assert.equal(hasFilters(f), true);
  const bad = parseFilters(new URLSearchParams("kind=service&activity=idle"));
  assert.equal(bad.kind, "");
  assert.equal(bad.activity, "");
  assert.equal(hasFilters(bad), false);
  assert.equal(filtersQuery({ q: "  ", kind: "", label: "", activity: "" }), "");
});

test("open nodes are unique and capped", () => {
  assert.deepEqual(parseOpen("a,b,a,,c"), ["a", "b", "c"]);
  assert.equal(parseOpen(Array.from({ length: 60 }, (_, i) => `n${i}`).join(",")).length, 50);
  assert.equal(openQuery(["a", "b"]), "a,b");
  assert.deepEqual(toggleOpen(["a", "b"], "a"), ["b"]);
  assert.deepEqual(toggleOpen(["a"], "b"), ["a", "b"]);
});

test("lazy rows descend only into open loaded branches", () => {
  const by = new Map([
    ["", branch([row("acme", null, 2), row("zeta", null, 1)])],
    ["acme", branch([row("backend", "acme", 1)], 60)],
  ]);
  const rows = buildRows(by, new Set(["acme", "zeta"]), "", true);
  assert.deepEqual(
    rows.map((r) => (r.type === "node" ? `${r.level}:${r.row.id}:${r.posinset}/${r.setsize}:${r.expanded}` : `${r.level}:${r.type}`)),
    ["1:acme:1/2:true", "2:backend:1/60:false", "2:more", "1:zeta:2/2:true", "2:loading"],
  );
});

test("a failed branch shows an error row", () => {
  const by = new Map([["", branch([row("acme", null, 1)])], ["acme", branch([], 0, { error: "boom" })]]);
  const rows = buildRows(by, new Set(["acme"]), "", true);
  assert.equal(rows[1].type, "error");
  assert.equal(rows[1].error, "boom");
});

test("filtered rows are grouped by parent and expand only where children came back", () => {
  const by = groupByParent([row("acme", null, 3, { match: false }), row("backend", "acme", 5), row("api", "backend", 0)], "");
  const rows = buildRows(by, new Set(["acme", "backend"]), "", false);
  assert.deepEqual(rows.map((r) => `${r.level}:${r.row.id}:${r.expandable}`), ["1:acme:true", "2:backend:true", "3:api:false"]);
});

test("highlight marks every case-insensitive match", () => {
  assert.deepEqual(highlight("Billing bill", "BILL"), [
    { text: "Bill", hit: true },
    { text: "ing ", hit: false },
    { text: "bill", hit: true },
  ]);
  assert.deepEqual(highlight("api", ""), [{ text: "api", hit: false }]);
});
