import assert from "node:assert/strict";
import { test } from "node:test";

import {
  canSubmitNodes,
  scopesForAccess,
  showsCatalogAdmin,
  TOKEN_MAX_NODES,
  tokenNodeLabels,
  tokenNodesBody,
  toggleScope,
} from "../src/lib/tokenNodes.ts";

const picked = (n) => Array.from({ length: n }, (_, i) => ({ id: `id-${i}`, label: `acme / n${i}` }));

test("the body carries nodes only for selected access", () => {
  assert.equal(tokenNodesBody("all", picked(2)), null);
  assert.deepEqual(tokenNodesBody("nodes", picked(2)), ["id-0", "id-1"]);
});

test("selected access needs a node and no whole-installation right", () => {
  assert.equal(canSubmitNodes("all", [], ["users:read"]), true);
  assert.equal(canSubmitNodes("nodes", [], ["read"]), false);
  assert.equal(canSubmitNodes("nodes", picked(1), ["read"]), true);
  assert.equal(canSubmitNodes("nodes", picked(1), ["read", "catalog.root"]), true);
  assert.equal(canSubmitNodes("nodes", picked(1), ["read", "users:read"]), false);
  assert.equal(canSubmitNodes("nodes", picked(TOKEN_MAX_NODES + 1), ["read"]), false);
});

test("choosing nodes drops the global rights but catalog.root", () => {
  assert.deepEqual(scopesForAccess("nodes", ["read", "mcp", "users:read", "settings:write", "catalog.root"]), ["read", "mcp", "catalog.root"]);
  assert.deepEqual(scopesForAccess("all", ["read", "settings:read"]), ["read", "settings:read"]);
});

test("a write right brings its read right, which stays removable", () => {
  assert.deepEqual(toggleScope(["read"], "users:write", true), ["read", "users:read", "users:write"]);
  assert.deepEqual(toggleScope(["read", "users:read", "users:write"], "users:read", false), ["read", "users:write"]);
  assert.deepEqual(toggleScope(["read", "users:read", "users:write"], "users:write", false), ["read", "users:read"]);
  assert.deepEqual(toggleScope(["read"], "catalog.root", true), ["read", "catalog.root"]);
});

test("the catalog admin label needs the flag and no whole-installation power", () => {
  assert.equal(showsCatalogAdmin({ catalog_admin: true, scopes: ["read", "write"], nodes: null }), true);
  assert.equal(showsCatalogAdmin({ catalog_admin: true, scopes: ["read", "clusters:read"], nodes: null }), false);
  assert.equal(showsCatalogAdmin({ catalog_admin: true, scopes: ["write", "catalog.root"], nodes: [] }), true);
  assert.equal(showsCatalogAdmin({ catalog_admin: false, scopes: ["read"], nodes: null }), false);
});

test("the nodes column names the whole catalog, paths and deleted nodes", () => {
  assert.deepEqual(tokenNodeLabels({ nodes: null }, "Whole catalog", "Deleted node"), ["Whole catalog"]);
  const nodes = [
    { id: "a", kind: null, name: null, path: null },
    { id: "b", kind: "folder", name: "Backend", path: "acme/backend" },
  ];
  assert.deepEqual(tokenNodeLabels({ nodes }, "Whole catalog", "Deleted node"), ["Deleted node", "acme/backend"]);
});
