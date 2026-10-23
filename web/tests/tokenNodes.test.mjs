import assert from "node:assert/strict";
import { test } from "node:test";

import { canSubmitNodes, scopesForAccess, TOKEN_MAX_NODES, tokenNodeLabels, tokenNodesBody } from "../src/lib/tokenNodes.ts";

const picked = (n) => Array.from({ length: n }, (_, i) => ({ id: `id-${i}`, label: `acme / n${i}` }));

test("the body carries nodes only for selected access", () => {
  assert.equal(tokenNodesBody("all", picked(2)), null);
  assert.deepEqual(tokenNodesBody("nodes", picked(2)), ["id-0", "id-1"]);
});

test("selected access needs a node and no admin scope", () => {
  assert.equal(canSubmitNodes("all", [], true), true);
  assert.equal(canSubmitNodes("nodes", [], false), false);
  assert.equal(canSubmitNodes("nodes", picked(1), false), true);
  assert.equal(canSubmitNodes("nodes", picked(1), true), false);
  assert.equal(canSubmitNodes("nodes", picked(TOKEN_MAX_NODES + 1), false), false);
});

test("choosing nodes drops the admin scope", () => {
  assert.deepEqual(scopesForAccess("nodes", ["read", "admin", "mcp"]), ["read", "mcp"]);
  assert.deepEqual(scopesForAccess("all", ["read", "admin"]), ["read", "admin"]);
});

test("the nodes column names the whole catalog, paths and deleted nodes", () => {
  assert.deepEqual(tokenNodeLabels({ nodes: null }, "Whole catalog", "Deleted node"), ["Whole catalog"]);
  const nodes = [
    { id: "a", kind: null, name: null, path: null },
    { id: "b", kind: "folder", name: "Backend", path: "acme/backend" },
  ];
  assert.deepEqual(tokenNodeLabels({ nodes }, "Whole catalog", "Deleted node"), ["Deleted node", "acme/backend"]);
});
