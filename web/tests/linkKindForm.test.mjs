import assert from "node:assert/strict";
import { test } from "node:test";

import { copySource, moveKey, namesBody, searchIcons } from "../src/lib/linkKindForm.ts";

test("empty names are copied from the first given one in en, es, ru, zh order", () => {
  assert.equal(copySource({ ru: "Пайплайн", en: " " }), "Пайплайн");
  assert.equal(copySource({ es: "Registros", ru: "Логи" }), "Registros");
  assert.equal(copySource({}), "");
});

test("only non-empty trimmed names are sent", () => {
  assert.deepEqual(namesBody({ en: "", es: "  ", ru: " Логи ", zh: "日志" }), { ru: "Логи", zh: "日志" });
});

test("moving a key keeps the others in order", () => {
  assert.deepEqual(moveKey(["a", "b", "c", "d"], 3, 0), ["d", "a", "b", "c"]);
  assert.deepEqual(moveKey(["a", "b", "c"], 0, 1), ["b", "a", "c"]);
  assert.deepEqual(moveKey(["a", "b"], 0, 5), ["a", "b"]);
});

test("icon search prefers names starting with the query and is capped", () => {
  const names = ["bug", "debug", "git-branch", "git-pull-request", "github", "logs"];
  assert.deepEqual(searchIcons(names, "git"), ["git-branch", "git-pull-request", "github"]);
  assert.deepEqual(searchIcons(names, "bug"), ["bug", "debug"]);
  assert.deepEqual(searchIcons(names, "pull request"), ["git-pull-request"]);
  assert.equal(searchIcons(names, "", 2).length, 2);
});
