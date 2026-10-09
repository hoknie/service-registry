import assert from "node:assert/strict";
import { test } from "node:test";

import { childrenOf, filterNodes, index, path, siblings } from "../src/lib/catalogNav.ts";

const node = (id, parent_id, name, slug = name.toLowerCase()) => ({ id, parent_id, name, slug, kind: "folder", access: "read" });
const nodes = [
  node("acme", null, "Acme"),
  node("globex", null, "Globex"),
  node("backend", "acme", "Backend"),
  node("frontend", "acme", "Frontend"),
  node("users", "backend", "Users API", "users-api"),
  node("billing", "backend", "Billing API", "bill"),
  node("orphan", "invisible", "Orphan"),
];
const ix = index(nodes);
const ids = (list) => list.map((n) => n.id);

test("siblings of a top-level node are the roots, sorted by name; a node whose parent is not loaded is a root", () => {
  assert.deepEqual(ids(siblings(ix, "globex")), ["acme", "globex", "orphan"]);
});

test("siblings of a nested node are the children of its parent", () => {
  assert.deepEqual(ids(siblings(ix, "billing")), ["billing", "users"]);
});

test("children and the path from the root", () => {
  assert.deepEqual(ids(childrenOf(ix, "acme")), ["backend", "frontend"]);
  assert.deepEqual(ids(path(ix, "users")), ["acme", "backend", "users"]);
});

test("filter by name or slug ignores case", () => {
  assert.deepEqual(ids(filterNodes(childrenOf(ix, "backend"), "BILL")), ["billing"]);
  assert.deepEqual(ids(filterNodes(childrenOf(ix, "backend"), "users-")), ["users"]);
  assert.equal(filterNodes(childrenOf(ix, "backend"), " ").length, 2);
});

test("an unknown id has no path, siblings or children", () => {
  assert.deepEqual(path(ix, "nope"), []);
  assert.deepEqual(siblings(ix, "nope"), []);
  assert.deepEqual(childrenOf(ix, "nope"), []);
});
