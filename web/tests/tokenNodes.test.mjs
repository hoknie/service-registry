import assert from "node:assert/strict";
import { test } from "node:test";

import { canSubmitNodes, scopesForAccess, showsCatalogAdmin, TOKEN_MAX_NODES, tokenNodeLabels, tokenNodesBody } from "../src/lib/tokenNodes.ts";

const picked = (n) => Array.from({ length: n }, (_, i) => ({ id: `id-${i}`, label: `acme / n${i}` }));

test("the body carries nodes only for selected access", () => {
  assert.equal(tokenNodesBody("all", picked(2)), null);
  assert.deepEqual(tokenNodesBody("nodes", picked(2)), ["id-0", "id-1"]);
});

test("selected access needs a node and no global right", () => {
  assert.equal(canSubmitNodes("all", [], true), true);
  assert.equal(canSubmitNodes("nodes", [], false), false);
  assert.equal(canSubmitNodes("nodes", picked(1), false), true);
  assert.equal(canSubmitNodes("nodes", picked(1), true), false);
  assert.equal(canSubmitNodes("nodes", picked(TOKEN_MAX_NODES + 1), false), false);
});

test("choosing nodes drops the global rights", () => {
  assert.deepEqual(scopesForAccess("nodes", ["read", "mcp", "users", "catalog.root"]), ["read", "mcp"]);
  assert.deepEqual(scopesForAccess("all", ["read", "settings"]), ["read", "settings"]);
});

test("the catalog admin label needs the flag and no global right", () => {
  assert.equal(showsCatalogAdmin({ catalog_admin: true, scopes: ["read", "write"] }), true);
  assert.equal(showsCatalogAdmin({ catalog_admin: true, scopes: ["read", "clusters"] }), false);
  assert.equal(showsCatalogAdmin({ catalog_admin: false, scopes: ["read"] }), false);
});

test("the nodes column names the whole catalog, paths and deleted nodes", () => {
  assert.deepEqual(tokenNodeLabels({ nodes: null }, "Whole catalog", "Deleted node"), ["Whole catalog"]);
  const nodes = [
    { id: "a", kind: null, name: null, path: null },
    { id: "b", kind: "folder", name: "Backend", path: "acme/backend" },
  ];
  assert.deepEqual(tokenNodeLabels({ nodes }, "Whole catalog", "Deleted node"), ["Deleted node", "acme/backend"]);
});
