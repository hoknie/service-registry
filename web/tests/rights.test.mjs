import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

import { visibleAdminGroups } from "../src/lib/admin.ts";
import { adminVisible, can, normalizeRights, RIGHT_SECTIONS, scopeAllowed, sectionVisible } from "../src/lib/rights.ts";

const bob = { is_superadmin: false, rights: { clusters: ["read"], environments: ["update"] } };

test("every section has a name in every locale", () => {
  for (const locale of ["en", "es", "ru", "zh"]) {
    const m = JSON.parse(readFileSync(new URL(`../src/i18n/messages/${locale}.json`, import.meta.url), "utf8"));
    assert.deepEqual(Object.keys(m.admin.groups.permissions.sections), RIGHT_SECTIONS, locale);
  }
});

test("rights decide actions and visible sections", () => {
  assert.equal(can(bob, "clusters", "read"), true);
  assert.equal(can(bob, "clusters", "delete"), false);
  assert.equal(can({ is_superadmin: true }, "users", "create"), true);
  assert.equal(sectionVisible(bob, "environments"), true);
  assert.equal(sectionVisible({ is_superadmin: false, rights: { secrets: ["update"] } }, "secrets"), false);
  assert.equal(adminVisible({ is_superadmin: false, rights: {} }), false);
  assert.deepEqual(
    visibleAdminGroups((s) => sectionVisible(bob, s)).map((g) => [g.key, g.sections.map((s) => s.path)]),
    [
      ["catalog", ["environments"]],
      ["infra", ["clusters"]],
    ],
  );
});

test("normalized rights keep applicable actions in order", () => {
  assert.deepEqual(normalizeRights({ clusters: ["update", "read"], scans: ["delete"], users: [] }), { clusters: ["read", "update"] });
});

test("global scopes follow the rights of their part", () => {
  assert.equal(scopeAllowed(bob, "clusters:read"), true);
  assert.equal(scopeAllowed(bob, "clusters:write"), false);
  assert.equal(scopeAllowed(bob, "settings:write"), true);
  assert.equal(scopeAllowed(bob, "settings:read"), false);
  assert.equal(scopeAllowed(bob, "catalog.root"), false);
  assert.equal(scopeAllowed({ is_superadmin: true }, "catalog.root"), true);
});

test("catalog read alone does not allow catalog.root", () => {
  assert.equal(scopeAllowed({ is_superadmin: false, rights: { catalog: ["read"] } }, "catalog.root"), false);
  assert.equal(scopeAllowed({ is_superadmin: false, rights: { catalog: ["read", "update"] } }, "catalog.root"), true);
  assert.deepEqual(normalizeRights({ catalog: ["delete", "read"] }), { catalog: ["read", "delete"] });
});

test("the publication right opens its token scope only", async () => {
  const { scopeAllowed, SECTION_ACTIONS, adminVisible } = await import("../src/lib/rights.ts");
  const user = { is_superadmin: false, rights: { publication: ["update"] } };
  assert.deepEqual(SECTION_ACTIONS.publication, ["update"]);
  assert.equal(scopeAllowed(user, "publication:write"), true);
  assert.equal(scopeAllowed(user, "settings:write"), false);
  assert.equal(scopeAllowed({ is_superadmin: false, rights: {} }, "publication:write"), false);
  if (adminVisible) assert.equal(adminVisible(user), false);
});

test("the skills right opens skills:write and no admin section", () => {
  const user = { is_superadmin: false, rights: { skills: ["create", "update"] } };
  assert.equal(scopeAllowed(user, "skills:write"), true);
  assert.equal(scopeAllowed({ is_superadmin: false, rights: {} }, "skills:write"), false);
  assert.equal(can(user, "skills", "delete"), false);
  assert.equal(adminVisible(user), false);
  assert.deepEqual(normalizeRights({ skills: ["delete", "create"] }), { skills: ["create", "delete"] });
});

test("the mcp right opens mcp_servers:write and no admin section", () => {
  const user = { is_superadmin: false, rights: { mcp: ["update"] } };
  assert.equal(scopeAllowed(user, "mcp_servers:write"), true);
  assert.equal(scopeAllowed({ is_superadmin: false, rights: {} }, "mcp_servers:write"), false);
  assert.equal(adminVisible(user), false);
  assert.deepEqual(normalizeRights({ mcp: ["update", "create"] }), { mcp: ["create", "update"] });
});
