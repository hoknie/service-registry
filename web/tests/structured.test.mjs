import assert from "node:assert/strict";
import { test } from "node:test";

import { entries, leafText, parseJson, structuredKind, valueType } from "../src/lib/structured.ts";

test("JSON and YAML files are recognized by their extension", () => {
  assert.equal(structuredKind("api/openapi.json"), "json");
  assert.equal(structuredKind("config.YAML"), "yaml");
  assert.equal(structuredKind(".github/ci.yml"), "yaml");
  assert.equal(structuredKind("README.md"), null);
  assert.equal(structuredKind("data.jsonl"), null);
});

test("values have a type, containers have entries", () => {
  const doc = parseJson('{"a": [1, "x", true, null], "b": {"c": 2}}')[0];
  assert.equal(valueType(doc), "object");
  assert.deepEqual(entries(doc).map(([k]) => k), ["a", "b"]);
  const a = entries(doc)[0][1];
  assert.equal(valueType(a), "array");
  assert.deepEqual(entries(a).map(([k, v]) => `${k}:${valueType(v)}`), ["0:number", "1:string", "2:boolean", "3:null"]);
  assert.equal(leafText("x"), '"x"');
  assert.equal(leafText(null), "null");
  assert.equal(valueType(new Date(0)), "string");
});

test("broken JSON throws", () => {
  assert.throws(() => parseJson('{"a": '));
});
