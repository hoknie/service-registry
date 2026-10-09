import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { test } from "node:test";

const dir = new URL("../src/i18n/messages/", import.meta.url);
const LOCALES = ["en", "es", "ru", "zh"];

function leaves(value, prefix = "") {
  if (typeof value === "string") return [[prefix, value]];
  assert.ok(value && typeof value === "object" && !Array.isArray(value), `${prefix}: object or string`);
  return Object.entries(value).flatMap(([k, v]) => leaves(v, prefix ? `${prefix}.${k}` : k));
}

const load = (locale) => JSON.parse(readFileSync(new URL(`${locale}.json`, dir), "utf8"));

test("exactly the four locale files exist", () => {
  const files = readdirSync(dir).filter((f) => f.endsWith(".json")).sort();
  assert.deepEqual(files, LOCALES.map((l) => `${l}.json`).sort());
});

test("all locales have the same keys as en", () => {
  const reference = leaves(load("en")).map(([k]) => k).sort();
  for (const locale of LOCALES) {
    const keys = leaves(load(locale)).map(([k]) => k).sort();
    assert.deepEqual(keys, reference, `${locale}.json keys differ from en.json`);
  }
});

test("no empty strings", () => {
  for (const locale of LOCALES) {
    for (const [key, text] of leaves(load(locale))) {
      assert.ok(text.trim().length > 0, `${locale}: ${key} is empty`);
    }
  }
});
