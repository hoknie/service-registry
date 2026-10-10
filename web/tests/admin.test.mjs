import assert from "node:assert/strict";
import { readdirSync } from "node:fs";
import { test } from "node:test";

import { ADMIN_GROUPS, ADMIN_PATHS, currentAdminGroup, currentAdminPath } from "../src/lib/admin.ts";

test("every admin page is in the menu, once", () => {
  const pages = readdirSync(new URL("../src/app/[locale]/admin/", import.meta.url), { withFileTypes: true })
    .filter((d) => d.isDirectory())
    .map((d) => d.name)
    .sort();
  assert.deepEqual([...ADMIN_PATHS].sort(), pages);
  assert.equal(new Set(ADMIN_PATHS).size, ADMIN_PATHS.length);
});

test("groups keep their order", () => {
  assert.deepEqual(
    ADMIN_GROUPS.map((g) => g.key),
    ["access", "secrets", "catalog", "infra", "docs"],
  );
  assert.equal(ADMIN_PATHS[0], "users");
});

test("the current section comes from the address", () => {
  assert.equal(currentAdminPath("/ru/admin/clusters", "ru"), "clusters");
  assert.equal(currentAdminPath("/ru/admin/users/user", "ru"), "users");
  assert.equal(currentAdminPath("/ru/admin", "ru"), null);
  assert.equal(currentAdminPath("/en/catalog", "en"), null);
});

test("the current group holds the current section, the root opens the first group", () => {
  assert.equal(currentAdminGroup("clusters").key, "infra");
  assert.equal(currentAdminGroup("environments").key, "catalog");
  assert.equal(currentAdminGroup(null).key, "access");
});
