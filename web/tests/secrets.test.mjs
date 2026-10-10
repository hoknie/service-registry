import assert from "node:assert/strict";
import { test } from "node:test";

import { credentialsText, NO_SECRET, secretBody, secretFrom, secretOptions, secretsPath } from "../src/lib/secrets.ts";

const t = { here: "Here", global: "Global", inherited: "From {name}", none: "No token", legacyShort: "Inline token" };
const own = { id: "a", name: "gitlab", description: "ci", own: true, from: { id: "n", name: "Acme" }, fingerprint: "ab12", ref: null };
const up = { id: "b", name: "deploy", description: "", own: false, from: { id: "p", name: "Root" }, fingerprint: null, ref: "env:X" };
const glob = { id: "c", name: "k8s", description: "", own: false, from: null, fingerprint: "ff", ref: null };

test("paths of node and global secrets", () => {
  assert.equal(secretsPath("n1"), "/v1/catalog/nodes/n1/secrets");
  assert.equal(secretsPath(null), "/v1/secrets");
});

test("where a secret comes from", () => {
  assert.equal(secretFrom(own, t), "Here");
  assert.equal(secretFrom(up, t), "From Root");
  assert.equal(secretFrom(glob, t), "Global");
});

test("picker options carry origin and storage, with an optional none", () => {
  const opts = secretOptions([own, up], t, true);
  assert.equal(opts[0].value, NO_SECRET);
  assert.deepEqual(opts[1], { value: "a", label: "gitlab", detail: "Here · ab12", keywords: ["ci"] });
  assert.equal(opts[2].detail, "From Root · env:X");
  assert.equal(secretOptions([glob], t, false).length, 1);
});

test("an edit without a new value keeps the old one", () => {
  const d = { name: " gl ", description: "", mode: "value", value: "", ref: "" };
  assert.deepEqual(secretBody(d, true), { name: "gl", description: "" });
  assert.deepEqual(secretBody(d, false), { name: "gl", description: "", value: "" });
  assert.deepEqual(secretBody({ ...d, mode: "ref", ref: " env:T " }, true), { name: "gl", description: "", ref: "env:T" });
});

test("credentials of a record read as text", () => {
  assert.equal(credentialsText({ kind: "secret", secret: { id: "c", name: "k8s", from: null } }, t), "k8s · Global");
  assert.equal(credentialsText({ kind: "secret", secret: { id: "a", name: "gl", from: { id: "n", name: "Acme" } } }, t), "gl · Acme");
  assert.equal(credentialsText({ kind: "legacy", storage: "stored", fingerprint: "x", ref: null }, t), "Inline token");
  assert.equal(credentialsText({ kind: "none" }, t), "No token");
});
