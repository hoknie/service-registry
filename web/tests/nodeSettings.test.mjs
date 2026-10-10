import assert from "node:assert/strict";
import { test } from "node:test";

import { resolveTab, sectionsOf, tabsOf } from "../src/lib/nodeSettings.ts";

const project = (permissions = []) => ({ kind: "project", access: "read", permissions });
const folder = (permissions = []) => ({ kind: "folder", access: "read", permissions });

test("tabs of a project and of a container", () => {
  assert.deepEqual(tabsOf(project()), ["overview", "about", "deployments", "events", "docs", "settings"]);
  assert.deepEqual(tabsOf(folder()), ["overview", "details", "settings"]);
  assert.deepEqual(tabsOf({ kind: "folder", access: "navigate" }), ["overview"]);
  assert.deepEqual(tabsOf(null), ["overview"]);
});

test("sections follow permissions", () => {
  assert.deepEqual(sectionsOf(project(["catalog.read"])), ["general", "branches", "links", "docs", "connect"]);
  assert.deepEqual(sectionsOf(project(["catalog.keys", "catalog.access"])), ["general", "branches", "links", "docs", "keys", "access", "connect"]);
  assert.deepEqual(sectionsOf(project(["catalog.write"])), ["general", "branches", "links", "docs", "scans", "connect"]);
  assert.deepEqual(sectionsOf(folder(["catalog.access"])), ["general", "forge", "secrets", "links", "docs", "access"]);
  assert.deepEqual(sectionsOf(folder(["catalog.read"])), ["general", "forge", "secrets", "links", "docs"]);
});

test("old tabs move into settings", () => {
  assert.deepEqual(resolveTab(project(), "branches", null), { tab: "settings", section: "branches", redirect: { tab: "settings", section: "branches" } });
  assert.deepEqual(resolveTab(project(), "keys", null).section, "general", "no right to keys");
  assert.deepEqual(resolveTab(folder(), "docs-settings", null).section, "docs");
  assert.deepEqual(resolveTab(project(), "settings", "access"), { tab: "settings", section: "general", redirect: null });
  assert.deepEqual(resolveTab(project(), "events", null), { tab: "events", section: null, redirect: null });
  assert.deepEqual(resolveTab(folder(), "events", null).tab, "overview");
  assert.deepEqual(resolveTab(folder(), "details", null), { tab: "details", section: null, redirect: null });
  assert.deepEqual(resolveTab(project(), "details", null), { tab: "overview", section: null, redirect: null }, "projects keep details on the overview");
  assert.deepEqual(resolveTab(project(), "settings", "secrets").section, "general", "projects have no secrets");
  assert.deepEqual(resolveTab(project(), "settings", "scans").section, "general", "viewers have no scan history");
  assert.deepEqual(resolveTab(project(["catalog.write"]), "settings", "scans").section, "scans");
});
