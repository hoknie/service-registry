import assert from "node:assert/strict";
import { test } from "node:test";

import { formatDuration, readScanParams, scanAddress, scanCodeText, scanCodes, scanQuery, scanTotals } from "../src/lib/scans.ts";

const book = {
  collect: {
    no_files_matched: { title: "No files matched", text: "t", hint: "Check the patterns", hintGit: "Commit the files" },
  },
  source: { not_found: { title: "Source missing", text: "t", hint: "h" } },
};

test("a known code reads from the book, git gets its own hint", () => {
  assert.equal(scanCodeText(book, "collect.no_files_matched", "local_dir").hint, "Check the patterns");
  assert.equal(scanCodeText(book, "collect.no_files_matched", "local_git").hint, "Commit the files");
  assert.equal(scanCodeText(book, "source.not_found", "local_git").hint, "h");
});

test("an unknown code is shown as is", () => {
  assert.deepEqual(scanCodeText(book, "forge.brand_new", null), { title: "forge.brand_new", text: "", hint: "" });
  assert.equal(scanCodeText(book, "internal", null).title, "internal");
});

const scan = {
  error: { code: "source.not_found", detail: "" },
  branches: [
    { name: "main", files: 3, skipped: { too_large: 2 }, error: "source.not_found" },
    { name: "dev", files: 1, skipped: {}, error: null },
  ],
  warnings: ["collect.files_skipped"],
};

test("codes are listed once, error first", () => {
  assert.deepEqual(scanCodes(scan), ["source.not_found", "collect.files_skipped"]);
  assert.deepEqual(scanTotals(scan), { branches: 2, files: 4, skipped: 2 });
});

test("params survive the address and become the API query", () => {
  const p = readScanParams((n) => ({ kind: "collect", status: "failed,bogus,warning", page: "3", trigger: "cron" })[n] ?? null);
  assert.deepEqual(p, { project: "", kind: "collect", status: ["failed", "warning"], trigger: "", page: 3 });
  assert.equal(scanAddress(p), "kind=collect&status=failed%2Cwarning&page=3");
  assert.equal(scanQuery(p), "kind=collect&status=failed%2Cwarning&limit=50&offset=100");
  assert.equal(readScanParams(() => null).page, 1);
});

test("durations", () => {
  assert.match(formatDuration(350, "en"), /350/);
  assert.match(formatDuration(1500, "en"), /1\.5/);
});
