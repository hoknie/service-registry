import assert from "node:assert/strict";
import { test } from "node:test";

import { forgetRecentNode, recentNodes, recordRecentNode, RECENT_NODES_KEY } from "../src/lib/recentNodes.ts";

const memory = () => {
  const data = new Map();
  return { getItem: (k) => data.get(k) ?? null, setItem: (k, v) => data.set(k, String(v)), data };
};
const node = (id, extra = {}) => ({ id, name: id.toUpperCase(), kind: "project", path: "Acme / Backend", ...extra });

test("newest first, without repeats, at most ten", () => {
  const s = memory();
  recordRecentNode(node("billing"), s);
  recordRecentNode(node("auth"), s);
  recordRecentNode(node("billing", { name: "Billing API" }), s);
  assert.deepEqual(recentNodes(s).map((r) => r.id), ["billing", "auth"]);
  assert.equal(recentNodes(s)[0].name, "Billing API");
  for (let i = 0; i < 12; i++) recordRecentNode(node(`p${i}`), s);
  assert.equal(recentNodes(s).length, 10);
  assert.equal(recentNodes(s)[0].id, "p11");
});

test("keeps only the stored fields", () => {
  const s = memory();
  recordRecentNode({ ...node("billing"), access: "read", labels: { a: "b" } }, s);
  assert.deepEqual(JSON.parse(s.data.get(RECENT_NODES_KEY)), [node("billing")]);
});

test("a removed node is forgotten", () => {
  const s = memory();
  recordRecentNode(node("billing"), s);
  recordRecentNode(node("auth"), s);
  forgetRecentNode("billing", s);
  assert.deepEqual(recentNodes(s).map((r) => r.id), ["auth"]);
});

test("broken or unavailable storage gives an empty list", () => {
  const throwing = { getItem: () => { throw new Error("denied"); }, setItem: () => { throw new Error("denied"); } };
  assert.deepEqual(recentNodes(throwing), []);
  recordRecentNode(node("billing"), throwing);
  forgetRecentNode("billing", throwing);
  assert.deepEqual(recentNodes(null), []);
  const s = memory();
  s.setItem(RECENT_NODES_KEY, "{not json");
  assert.deepEqual(recentNodes(s), []);
  s.setItem(RECENT_NODES_KEY, JSON.stringify([{ id: 1 }, node("ok"), { ...node("bad"), kind: "service" }]));
  assert.deepEqual(recentNodes(s).map((r) => r.id), ["ok"]);
});
