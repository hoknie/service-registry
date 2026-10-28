import assert from "node:assert/strict";
import { test } from "node:test";

import {
  curlCommand,
  fileTree,
  filtersFrom,
  listQuery,
  placeFrom,
  placeHref,
  manifestVersion,
  nextPatch,
  resolveLink,
  validVersion,
} from "../src/lib/skills.ts";

const file = (path) => ({ path, size: 1, kind: "text", title: null });

test("the file tree puts SKILL.md first and folders before files", () => {
  const tree = fileTree([
    file("reference.md"),
    file("scripts/run.sh"),
    file("SKILL.md"),
    file("a.md"),
    file("scripts/lib/x.sh"),
  ]);
  assert.deepEqual(
    tree.map((n) => n.path),
    ["SKILL.md", "scripts", "a.md", "reference.md"],
  );
  assert.deepEqual(
    tree[1].children.map((n) => n.path),
    ["scripts/lib", "scripts/run.sh"],
  );
});

test("places and filters round-trip through the address", () => {
  const f = filtersFrom("?folder=x&q=go&tag=go&tag=testing");
  assert.deepEqual(f, { q: "go", tags: ["go", "testing"], status: "", active: false });
  assert.deepEqual(placeFrom("?space=global"), { folder: null, space: "global" });
  assert.equal(placeHref("en", { folder: "x", space: null }, f), "/en/skills?folder=x&q=go&tag=go&tag=testing");
  assert.equal(listQuery({ folder: "x", space: null }, f), "/v1/skills?folder_id=x&recursive=true&q=go&tag=go&tag=testing");
  assert.equal(listQuery({ folder: "x", space: null }, { q: "", tags: [], status: "" }), "/v1/skills?folder_id=x");
  assert.equal(listQuery({ folder: null, space: "org1" }, { q: "", tags: [], status: "" }), "/v1/skills?scope=org1&folder_id=root");
  assert.equal(listQuery({ folder: null, space: null }, { q: "go", tags: [], status: "" }), "/v1/skills?q=go");
});

test("the curl command names the version and the target", () => {
  assert.equal(
    curlCommand(
      "https://r.example",
      { id: "id1", name: "go-testing" },
      "1.3.0",
      "~/.claude/skills",
    ),
    'curl -fsSL -H "Authorization: Bearer $SVCR_TOKEN" "https://r.example/api/v1/skills/id1/archive?version=1.3.0" -o go-testing-1.3.0.zip && unzip -o go-testing-1.3.0.zip -d ~/.claude/skills',
  );
  assert.ok(
    curlCommand(
      "https://r",
      { id: "i", name: "n" },
      null,
      ".claude/skills",
    ).includes('/archive" -o n.zip'),
  );
});

test("versions are suggested from SKILL.md or the next patch", () => {
  assert.equal(nextPatch(null), "0.1.0");
  assert.equal(nextPatch("1.2.0"), "1.2.1");
  assert.equal(nextPatch("2.0.0-rc.1"), "2.0.0");
  assert.equal(
    manifestVersion("---\nname: x\nmetadata:\n  version: v1.4.0\n---\n# x"),
    "1.4.0",
  );
  assert.equal(manifestVersion("# no frontmatter"), null);
  assert.equal(validVersion("1.2"), false);
  assert.equal(validVersion("1.2.3-rc.1"), true);
});

test("links inside a skill resolve to its files", () => {
  const known = new Set(["reference.md", "scripts/run.sh", "docs/a.md"]);
  assert.equal(resolveLink("SKILL.md", "reference.md", known), "reference.md");
  assert.equal(
    resolveLink("docs/a.md", "../scripts/run.sh#L1", known),
    "scripts/run.sh",
  );
  assert.equal(resolveLink("SKILL.md", "https://x/y", known), null);
  assert.equal(resolveLink("SKILL.md", "../outside.md", known), null);
});

test("folder slugs come from the name, Cyrillic included", async () => {
  const { slugOf } = await import("../src/lib/slug.ts");
  assert.equal(slugOf("Go Testing"), "go-testing");
  assert.equal(slugOf("Тестирование и CI"), "testirovanie-i-ci");
  assert.equal(slugOf("Café — déjà vu!"), "cafe-deja-vu");
  assert.equal(slugOf("  ---  "), "");
  assert.equal(slugOf("a".repeat(63) + " b").length <= 64, true);
});
