import assert from "node:assert/strict";
import { test } from "node:test";

import { parseTreeLink } from "../src/lib/forgeAddress.ts";
import { archiveErrorCode, filtersFrom, folderCurlCommand, isOff, listQuery } from "../src/lib/skills.ts";

test("links to a directory split into the repository, branch and root", () => {
  assert.deepEqual(parseTreeLink("https://github.com/apollographql/skills/tree/main/skills"), {
    url: "https://github.com/apollographql/skills",
    branch: "main",
    root: "skills",
  });
  assert.deepEqual(parseTreeLink("https://github.com/Jeffallan/claude-skills/tree/main"), {
    url: "https://github.com/Jeffallan/claude-skills",
    branch: "main",
    root: "",
  });
  assert.deepEqual(parseTreeLink("https://gitlab.example.com/a/b/-/tree/dev/x/y"), {
    url: "https://gitlab.example.com/a/b",
    branch: "dev",
    root: "x/y",
  });
  assert.deepEqual(parseTreeLink("https://codeberg.org/acme/skills/src/branch/main/s"), {
    url: "https://codeberg.org/acme/skills",
    branch: "main",
    root: "s",
  });
  assert.equal(parseTreeLink("https://github.com/acme/skills"), null);
  assert.equal(parseTreeLink("not a url"), null);
});

test("folder install command and disabled marks", () => {
  assert.equal(
    folderCurlCommand("https://r.example", { id: "f1", slug: "apollo" }, "~/.claude/skills"),
    'curl -fsSL -H "Authorization: Bearer $SVCR_TOKEN" "https://r.example/api/v1/skills/folders/f1/archive" -o apollo.zip && unzip -o apollo.zip -d ~/.claude/skills',
  );
  assert.equal(isOff({ status: "disabled", disabled_by: null }), true);
  assert.equal(isOff({ status: "published", disabled_by: { id: "x", slug: "x", name: "X" } }), true);
  assert.equal(isOff({ status: "published", disabled_by: null }), false);
  assert.equal(archiveErrorCode(409), "conflict.skill_folder_empty");
  assert.equal(archiveErrorCode(413), "validation.skill_folder_too_large");
});

test("the active filter travels through the address and the query", () => {
  const f = filtersFrom("?active=true&q=go");
  assert.equal(f.active, true);
  assert.match(listQuery({ folder: "f", space: null }, f), /active=true/);
});
