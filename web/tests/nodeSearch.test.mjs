import assert from "node:assert/strict";
import { test } from "node:test";

import { nodeHits, nodeSearchQuery } from "../src/lib/nodeSearch.ts";

const row = (id, parent_id, name, match = true) => ({ id, parent_id, name, slug: id, kind: "folder", access: "read", children: 0, match, activity: [], links: [] });

test("an empty input asks nothing", () => {
  assert.equal(nodeSearchQuery(""), null);
  assert.equal(nodeSearchQuery("   "), null);
  assert.equal(nodeSearchQuery("a".repeat(101)), null);
  assert.equal(nodeSearchQuery(" 100% bill "), "/v1/catalog/table?q=100%25%20bill");
});

test("hits carry the path of their ancestors from the same answer", () => {
  const table = {
    items: [row("acme", null, "Acme", false), row("backend", "acme", "Backend", false), row("billing", "backend", "Billing API"), row("zeta", null, "Zeta")],
    total: 4,
    limit: 4,
    offset: 0,
    truncated: false,
  };
  assert.deepEqual(
    nodeHits(table).map((h) => [h.node.id, h.path]),
    [
      ["billing", "Acme / Backend"],
      ["zeta", ""],
    ],
  );
});

test("at most fifty hits", () => {
  const items = Array.from({ length: 60 }, (_, i) => row(`p${i}`, null, `P${i}`));
  assert.equal(nodeHits({ items, total: 60, limit: 60, offset: 0, truncated: false }).length, 50);
});
