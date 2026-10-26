import assert from "node:assert/strict";
import { test } from "node:test";

import { docHref, docPathCrumbs, publicRoot, setDocBase, fileName, isDirectory, isDocPath, parentDir, parseDocLocation, splitPath, startPage } from "../src/lib/docPath.ts";

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

test("doc locations and hrefs carry a tag", () => {
  assert.equal(parseDocLocation("/doc/a/docs/guide.md", "?tag=v1.2.0").branch, "tag:v1.2.0");
  assert.equal(parseDocLocation("/doc/a", "?branch=develop&tag=v1.2.0").branch, "tag:v1.2.0");
  assert.equal(docHref(["a", "docs", "guide.md"], "tag:v1.2.0"), "/doc/a/docs/guide.md?tag=v1.2.0");
  assert.equal(docHref(["a"], "develop"), "/doc/a?branch=develop");
});

test("directories and crumbs inside a project", () => {
  const paths = ["README.md", "docs/guide.md", "docs/api/v1.md"];
  assert.equal(isDirectory(paths, "docs"), true);
  assert.equal(isDirectory(paths, "docs/api"), true);
  assert.equal(isDirectory(paths, "docs/guide.md"), false);
  assert.equal(isDirectory(paths, "doc"), false);
  assert.equal(isDirectory(paths, null), false);
  assert.deepEqual(docPathCrumbs("docs/api/v1.md"), [
    { name: "docs", path: "docs" },
    { name: "api", path: "docs/api" },
    { name: "v1.md", path: "docs/api/v1.md" },
  ]);
  assert.equal(fileName("docs/api/v1.md"), "v1.md");
  assert.equal(parentDir("docs/api/v1.md"), "docs/api");
  assert.equal(parentDir("README.md"), "");
});

test("public bases", () => {
  assert.deepEqual(publicRoot("/public/docs/acme/backend"), { root: "acme", base: "/public/docs/acme" });
  assert.deepEqual(publicRoot("/public/docs"), { root: null, base: "/public/docs" });
  assert.deepEqual(publicRoot("/backend/api"), { root: null, base: "" });
  setDocBase("/public/docs/acme");
  assert.deepEqual(parseDocLocation("/public/docs/acme/backend/api", "?tag=v1"), { segments: ["backend", "api"], branch: "tag:v1" });
  assert.equal(docHref(["backend", "api"]), "/public/docs/acme/backend/api");
  assert.equal(docHref([]), "/public/docs/acme");
  setDocBase("");
  assert.deepEqual(parseDocLocation("/backend/api", "").segments, ["backend", "api"]);
  assert.equal(docHref(["backend"]), "/backend");
  assert.equal(docHref([]), "/");
  setDocBase("/doc");
});
