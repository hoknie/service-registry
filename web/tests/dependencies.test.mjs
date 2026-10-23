import assert from "node:assert/strict";
import { test } from "node:test";

import { automaticOf, depthOf, externalTarget, manualOf, targetKey, viewOf, withView } from "../src/lib/dependencies.ts";
import { placeNodes } from "../src/lib/dependencyLayout.ts";

test("view and depth come from the address", () => {
  assert.equal(viewOf(null, false), "list");
  assert.equal(viewOf("graph", false), "graph");
  assert.equal(viewOf(null, true), "graph");
  assert.equal(depthOf("2"), 2);
  assert.equal(depthOf("5"), 1);
  assert.equal(depthOf(null), 1);
  assert.equal(withView("/en/catalog?node=a&tab=dependencies", "list", 3), "/en/catalog?node=a&tab=dependencies");
  assert.equal(withView("/en/catalog?node=a&tab=dependencies", "graph", 1), "/en/catalog?node=a&tab=dependencies&view=graph");
  assert.equal(withView("/en/catalog?node=a&tab=dependencies", "graph", 2), "/en/catalog?node=a&tab=dependencies&view=graph&depth=2");
});

test("targets have stable keys and manual sources are separated", () => {
  assert.equal(targetKey({ type: "external", name: "redis" }), "external:redis");
  assert.equal(targetKey({ type: "unresolved", ref: "acme/x" }), "ref:acme/x");
  assert.equal(externalTarget("  Stripe "), "external:stripe");
  const items = [
    { target: { type: "project", id: "p", path: "acme/billing", name: "billing" }, sources: [
      { source: "event", origin: "api/production", seen_at: "2026-10-01T10:00:00Z" },
      { source: "manual", origin: "", seen_at: "2026-10-01T10:00:00Z", id: "m1", note: "pay" },
    ] },
  ];
  assert.deepEqual(manualOf(items).map((m) => m.source.id), ["m1"]);
  assert.deepEqual(automaticOf(items).map((m) => m.source.origin), ["api/production"]);
});

test("graph nodes are laid out left to right without overlap", () => {
  const placed = placeNodes({
    nodes: [
      { id: "a", type: "project", path: "acme/a", name: "a", in_scope: true },
      { id: "b", type: "project", path: "acme/b", name: "b", in_scope: false },
      { id: "external:x", type: "external", path: "", name: "x", in_scope: false },
    ],
    edges: [{ from: "a", to: "b", sources: ["event"] }, { from: "b", to: "external:x", sources: ["manual"] }],
    hidden_edges: 0,
    truncated: false,
  });
  const at = Object.fromEntries(placed.map((p) => [p.id, p]));
  assert.ok(at.a.x < at.b.x && at.b.x < at["external:x"].x);
});
