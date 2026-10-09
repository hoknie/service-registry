import assert from "node:assert/strict";
import { test } from "node:test";

import { addTags, fromTags, parseKeyValue, splitTags, toTags, validDnsLabel, validPattern } from "../src/lib/tags.ts";

test("pasted lines and commas split into tags, braces keep their commas", () => {
  assert.deepEqual(splitTags("docs/**\n *.md \r\n"), ["docs/**", "*.md"]);
  assert.deepEqual(splitTags("a, b,{x,y}/*.md"), ["a", "b", "{x,y}/*.md"]);
});

test("repeats are not added; text round-trips", () => {
  assert.deepEqual(addTags(["a"], ["a", "b", "b"]), ["a", "b"]);
  assert.deepEqual(toTags(fromTags(["a", "b"])), ["a", "b"]);
});

test("key=value pairs", () => {
  assert.deepEqual(parseKeyValue("tier = 1"), { key: "tier", value: "1" });
  assert.deepEqual(parseKeyValue("team="), { key: "team", value: "" });
  assert.equal(parseKeyValue("без знака равно"), null);
  assert.equal(parseKeyValue("=x"), null);
  assert.equal(parseKeyValue("two words=x"), null);
});

test("patterns and namespaces", () => {
  assert.equal(validPattern("docs/{a,b}/[a-z]*.md"), true);
  assert.equal(validPattern("docs/[a-"), false);
  assert.equal(validPattern("}{"), false);
  assert.equal(validDnsLabel("kube-system"), true);
  assert.equal(validDnsLabel("Kube"), false);
  assert.equal(validDnsLabel("-a"), false);
});

test("labels are a key or key=value", async () => {
  const { labelsOf, parseLabel } = await import("../src/lib/tags.ts");
  assert.deepEqual(parseLabel("critical"), { key: "critical", value: "" });
  assert.deepEqual(parseLabel("team = payments"), { key: "team", value: "payments" });
  assert.equal(parseLabel("Team"), null);
  assert.equal(parseLabel("-x"), null);
  assert.equal(parseLabel("a=" + "x".repeat(64)), null);
  assert.deepEqual(labelsOf("critical\nteam=payments", "tier=gold"), { critical: "", team: "payments", tier: "gold" });
});
