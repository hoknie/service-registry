import assert from "node:assert/strict";
import { test } from "node:test";

import { draftOf, publicationBody, unreachable } from "../src/lib/publication.ts";

const base = {
  publish: true,
  versions: [{ pattern: "v*", kind: "tag" }],
  default_branch: true,
  current_release: true,
  name: null,
  host: null,
  own: { versions: [{ pattern: "v*", kind: "tag" }], name: "acme-api" },
  from: { publish: { id: "o", kind: "organization", name: "acme" }, versions: null, default_branch: null, current_release: null },
  entries: [],
  can_edit: true,
};

test("draft and body keep only own fields", () => {
  const d = draftOf(base);
  assert.equal(d.publish.set, false);
  assert.equal(d.versions.set, true);
  assert.equal(d.name, "acme-api");
  d.versions.rows.push({ pattern: " ", kind: "branch" });
  assert.deepEqual(publicationBody(d, true), { versions: [{ pattern: "v*", kind: "tag" }], name: "acme-api" });
  assert.deepEqual(publicationBody(d, false), { versions: [{ pattern: "v*", kind: "tag" }] });
  d.publish = { set: true, value: false };
  d.host = " docs.acme.com ";
  assert.deepEqual(publicationBody(d, false), { publish: false, versions: [{ pattern: "v*", kind: "tag" }], host: "docs.acme.com" });
});

test("a published node without an address is unreachable", () => {
  assert.equal(unreachable(base), true);
  assert.equal(unreachable({ ...base, entries: [{ kind: "path", url: "/public/docs/acme" }] }), false);
  assert.equal(unreachable({ ...base, publish: false }), false);
});
