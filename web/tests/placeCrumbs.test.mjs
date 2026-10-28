import assert from "node:assert/strict";
import { test } from "node:test";

import { MENU_LIMIT, inSpace, placeEntries, placeLink } from "../src/lib/placeCrumbs.ts";

const crumb = (kind, extra = {}) => ({ section: "skills", kind, id: "f1", space: "acme", spaceName: "Acme", parent: null, global: "Global", ...extra });
const level = (more = {}) => ({ spaces: [], near: [], folders: [], items: [], up: null, ...more });

test("the root crumb lists the spaces", () => {
  const out = placeEntries(crumb("root", { id: "root" }), false, level({ spaces: [{ id: "global", name: "Global" }, { id: "acme", name: "Acme" }] }), "ru");
  assert.deepEqual(
    out.map((e) => [e.group, e.kind, e.href]),
    [
      ["inside", "space", "/ru/skills?space=global"],
      ["inside", "space", "/ru/skills?space=acme"],
    ],
  );
});

test("the last folder crumb shows up, next to it and inside", () => {
  const out = placeEntries(
    crumb("folder", { section: "mcp" }),
    true,
    level({ up: { kind: "space", ref: { id: "acme", name: "Acme" } }, near: [{ id: "f1", name: "Go" }, { id: "f2", name: "Python" }], folders: [{ id: "f3", name: "Testing" }], items: [{ id: "s1", name: "Jira" }] }),
    "en",
  );
  assert.deepEqual(
    out.map((e) => [e.group, e.kind, e.name, e.href]),
    [
      ["up", "space", "Acme", "/en/mcp?space=acme"],
      ["near", "folder", "Go", "/en/mcp?folder=f1"],
      ["near", "folder", "Python", "/en/mcp?folder=f2"],
      ["inside", "folder", "Testing", "/en/mcp?folder=f3"],
      ["inside", "item", "Jira", "/en/mcp/server?id=s1"],
    ],
  );
});

test("a crumb that is not last shows only its neighbours", () => {
  const out = placeEntries(crumb("space"), false, level({ spaces: [{ id: "acme", name: "Acme" }], folders: [{ id: "x", name: "X" }] }), "ru");
  assert.deepEqual(out.map((e) => e.group), ["near"]);
});

test("a group shows at most the menu limit", () => {
  const many = Array.from({ length: MENU_LIMIT + 5 }, (_, i) => ({ id: `f${i}`, name: `F${i}` }));
  assert.equal(placeEntries(crumb("folder"), false, level({ near: many }), "ru").length, MENU_LIMIT);
});

test("top folders are kept to their space", () => {
  const all = [
    { id: "a", name: "A", organization_id: null },
    { id: "b", name: "B", organization_id: "acme" },
  ];
  assert.deepEqual(inSpace(all, "global").map((r) => r.id), ["a"]);
  assert.deepEqual(inSpace(all, "acme").map((r) => r.id), ["b"]);
  assert.equal(placeLink("es", "skills", "item", "k"), "/es/skills/skill?id=k");
});

test("the trail names the section, the space and the folders", async () => {
  const { placeTrail } = await import("../src/lib/placeCrumbs.ts");
  const trail = placeTrail({ section: "skills", locale: "ru", title: "Навыки", global: "Глобальные", space: "acme", spaceName: "Acme", folders: [{ id: "a", name: "Go" }, { id: "b", name: "Testing" }], current: "folder" });
  assert.deepEqual(
    trail.map((c) => [c.label, c.href ?? null, c.place?.kind, c.place?.parent ?? null]),
    [
      ["Навыки", "/ru/skills", "root", null],
      ["Acme", "/ru/skills?space=acme", "space", null],
      ["Go", "/ru/skills?folder=a", "folder", null],
      ["Testing", null, "folder", "a"],
    ],
  );
  const item = placeTrail({ section: "mcp", locale: "en", title: "MCP", global: "Global", space: "global", spaceName: "Global", folders: [], current: "item" });
  assert.equal(item.at(-1).href, "/en/mcp?space=global");
});
