import assert from "node:assert/strict";
import { test } from "node:test";

import { resolveTab, sectionsOf, tabsOf } from "../src/lib/nodeSettings.ts";

const project = (permissions = []) => ({ kind: "project", access: "read", permissions });
const folder = (permissions = []) => ({ kind: "folder", parent_id: "parent", access: "read", permissions });
const organization = (permissions = []) => ({ kind: "organization", parent_id: null, access: "read", permissions });

test("tabs of a project and of a container", () => {
  assert.deepEqual(tabsOf(project()), ["overview", "docs", "details", "releases", "dependencies", "settings"]);
  assert.deepEqual(tabsOf(folder()), ["overview", "details", "dependencies", "settings"]);
  assert.deepEqual(tabsOf({ kind: "organization", access: "read" }), ["overview", "details", "dependencies", "members", "settings"]);
  assert.deepEqual(tabsOf({ kind: "organization", parent_id: "p", access: "read" }), ["overview", "details", "dependencies", "settings"]);
  assert.deepEqual(tabsOf({ kind: "folder", access: "navigate" }), ["overview"]);
  assert.deepEqual(tabsOf(null), ["overview"]);
});

test("sections follow permissions", () => {
  assert.deepEqual(sectionsOf(project(["catalog.read"])), ["general", "owners", "branches", "links", "docs", "publication", "polling", "connect"]);
  assert.deepEqual(sectionsOf(project(["catalog.keys", "catalog.access"])), ["general", "owners", "branches", "links", "docs", "publication", "polling", "keys", "access", "notifications", "connect"]);
  assert.deepEqual(sectionsOf(project(["catalog.write"])), ["general", "owners", "branches", "links", "dependencies", "docs", "publication", "polling", "scans", "connect"]);
  assert.deepEqual(sectionsOf(folder(["catalog.access"])), ["general", "owners", "repo-sync", "secrets", "links", "docs", "publication", "polling", "access", "notifications"]);
  assert.deepEqual(sectionsOf(folder(["catalog.read"])), ["general", "owners", "repo-sync", "secrets", "links", "docs", "publication", "polling"]);
  assert.deepEqual(sectionsOf(organization(["catalog.read"])), ["general", "owners", "groups", "repo-sync", "secrets", "links", "docs", "publication", "polling"]);
});

test("old tabs move into settings", () => {
  assert.deepEqual(resolveTab(project(), "branches", null), { tab: "settings", section: "branches", redirect: { tab: "settings", section: "branches" } });
  assert.deepEqual(resolveTab(project(), "keys", null).section, "general", "no right to keys");
  assert.deepEqual(resolveTab(folder(), "docs-settings", null).section, "docs");
  assert.deepEqual(resolveTab(folder(), "forge", null).section, "repo-sync");
  assert.deepEqual(resolveTab(folder(), "settings", "forge"), { tab: "settings", section: "repo-sync", redirect: { tab: "settings", section: "repo-sync" } });
  assert.deepEqual(resolveTab(project(), "settings", "access"), { tab: "settings", section: "general", redirect: null });
  assert.deepEqual(resolveTab(project(), "releases", null), { tab: "releases", section: null, redirect: null });
  assert.deepEqual(resolveTab(folder(), "events", null).tab, "overview");
  assert.deepEqual(resolveTab(folder(), "details", null), { tab: "details", section: null, redirect: null });
  assert.deepEqual(resolveTab(project(), "details", null), { tab: "details", section: null, redirect: null }, "projects have details too");
  assert.deepEqual(resolveTab(project(), "settings", "secrets").section, "general", "projects have no secrets");
  assert.deepEqual(resolveTab(project(), "settings", "scans").section, "general", "viewers have no scan history");
  assert.deepEqual(resolveTab(project(["catalog.write"]), "settings", "scans").section, "scans");
  assert.deepEqual(resolveTab(project(["catalog.write"]), "settings", "notifications").section, "general", "editors have no notifications");
  assert.deepEqual(resolveTab(project(["catalog.access"]), "settings", "notifications").section, "notifications");
});

test("renamed and removed project tabs redirect", () => {
  assert.deepEqual(resolveTab(project(), "deployments", null), { tab: "releases", section: null, redirect: { tab: "releases", section: null } });
  assert.deepEqual(resolveTab(project(), "about", null), { tab: "docs", section: null, redirect: { tab: "docs", section: null } });
  assert.deepEqual(resolveTab(project(), "events", null), { tab: "overview", section: null, redirect: { tab: "overview", section: null } });
  assert.deepEqual(resolveTab(folder(), "deployments", null), { tab: "overview", section: null, redirect: null });
});

test("organizations have no skills or MCP sections: they live in the spaces of those sections", () => {
  const org = { kind: "organization", parent_id: null, access: "read", permissions: ["catalog.read", "catalog.write", "catalog.access"] };
  assert.equal(sectionsOf(org).includes("skills"), false);
  assert.equal(sectionsOf(org).includes("mcp"), false);
  assert.equal(resolveTab(org, "settings", "skills").section, "general");
});
