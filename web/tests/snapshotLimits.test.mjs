import assert from "node:assert/strict";
import { test } from "node:test";

import { snapshotMissing } from "../src/lib/snapshotLimits.ts";

test("files left out by limits are counted, other skips are not", () => {
  assert.equal(snapshotMissing({ skipped_limit: 37, skipped_too_large: 2 }), 39);
  assert.equal(snapshotMissing({ skipped_limit: 0, skipped_too_large: 0 }), 0);
  assert.equal(snapshotMissing({}), 0);
});
