import assert from "node:assert/strict";
import { test } from "node:test";

import { docHref, isDocPath, parseDocLocation, splitPath, startPage } from "../src/lib/docPath.ts";

test("doc locations split into decoded segments and a branch", () => {
  assert.deepEqual(parseDocLocation("/doc", ""), { segments: [], branch: null });
  assert.deepEqual(parseDocLocation("/doc/rsqa/cs/route/route-prj/openspec/specs/Route.md", "?branch=develop"), {
    segments: ["rsqa", "cs", "route", "route-prj", "openspec", "specs", "Route.md"],
    branch: "develop",
  });
  assert.deepEqual(parseDocLocation("/doc/a/%D0%B4%D0%BE%D0%BA.md", "?branch=feature%2Fx").segments, ["a", "док.md"]);
  assert.equal(parseDocLocation("/doc/a", "?branch=feature%2Fx").branch, "feature/x");
  assert.deepEqual(parseDocLocation("/docs/a", "").segments, []);
});

test("doc hrefs encode each segment and the branch", () => {
  assert.equal(docHref([]), "/doc");
  assert.equal(docHref(["rsqa", "cs"]), "/doc/rsqa/cs");
  assert.equal(docHref(["a", "my file.md"], "feature/x"), "/doc/a/my%20file.md?branch=feature%2Fx");
  assert.equal(docHref(["a"], null), "/doc/a");
});

test("paths and the doc section", () => {
  assert.deepEqual(splitPath("openspec/specs/x.md"), ["openspec", "specs", "x.md"]);
  assert.equal(isDocPath("/doc"), true);
  assert.equal(isDocPath("/doc/a"), true);
  assert.equal(isDocPath("/docs"), false);
  assert.equal(isDocPath("/ru/catalog"), false);
});

test("the start page is the root README, else the first Markdown file", () => {
  assert.equal(startPage(["docs/a.md", "Readme.md", "README.md"]), "Readme.md");
  assert.equal(startPage(["docs/README.md", "z.md", "b/a.md"]), "b/a.md");
  assert.equal(startPage(["openapi.json"]), null);
});
