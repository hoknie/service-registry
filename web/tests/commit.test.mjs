import assert from "node:assert/strict";
import { test } from "node:test";

import { shortCommit } from "../src/lib/commit.ts";

test("a plain commit is cut to seven characters", () => {
  assert.deepEqual(shortCommit("a1b2c3d4e5f6a7b8"), { short: "a1b2c3d", worktree: false });
});

test("a working tree snapshot keeps the commit and is marked", () => {
  assert.deepEqual(shortCommit("a1b2c3d4e5f6+worktree:sha256:ffeeddcc"), { short: "a1b2c3d", worktree: true });
});

test("a local directory fingerprint shows its hash, not the prefix", () => {
  assert.deepEqual(shortCommit("sha256:0123456789abcdef"), { short: "0123456", worktree: false });
  assert.deepEqual(shortCommit(""), { short: "—", worktree: false });
});
