import assert from "node:assert/strict";
import { test } from "node:test";

import { isMarkdownReadme, isRootReadme, pickReadme } from "../src/lib/readmeFallback.ts";

test("only a README at the root counts, in any case and extension", () => {
  assert.equal(isRootReadme("README.md"), true);
  assert.equal(isRootReadme("readme"), true);
  assert.equal(isRootReadme("ReadMe.rst"), true);
  assert.equal(isRootReadme("docs/README.md"), false);
  assert.equal(isRootReadme("readme."), false);
  assert.equal(isRootReadme("READMEFIRST.md"), false);
});

test("README.md wins, otherwise the first by name", () => {
  const f = (path) => ({ path });
  assert.equal(pickReadme([f("readme.rst"), f("ReadMe.md"), f("docs/README.md")]).path, "ReadMe.md");
  assert.equal(pickReadme([f("README.txt"), f("README.rst")]).path, "README.rst");
  assert.equal(pickReadme([f("docs/README.md"), f("index.md")]), null);
});

test("markdown READMEs render, others are shown as text", () => {
  assert.equal(isMarkdownReadme("README.md"), true);
  assert.equal(isMarkdownReadme("README"), true);
  assert.equal(isMarkdownReadme("README.rst"), false);
});
