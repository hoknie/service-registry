import assert from "node:assert/strict";
import { test } from "node:test";

import { absoluteLink, displayName, membersQuery, providerStart, stateTone } from "../src/lib/members.ts";

test("a relative invitation link gets the origin of the page", () => {
  assert.equal(absoluteLink("/en/invite?token=svci_x", "http://127.0.0.1:8080/"), "http://127.0.0.1:8080/en/invite?token=svci_x");
  assert.equal(absoluteLink("https://registry.example/en/invite?token=svci_x", "http://x"), "https://registry.example/en/invite?token=svci_x");
});

test("provider sign-in carries the invitation and comes back to it", () => {
  assert.equal(
    providerStart("corp", "svci_x", "ru"),
    "/api/v1/auth/oauth/corp/start?invite=svci_x&next=%2Fru%2Finvite%3Ftoken%3Dsvci_x",
  );
});

test("queries, states and names", () => {
  assert.equal(membersQuery("  ann ", null), "?q=ann");
  assert.equal(membersQuery("", "c1"), "?cursor=c1");
  assert.equal(membersQuery("", null), "");
  assert.equal(stateTone("pending"), "signal");
  assert.equal(stateTone("expired"), "amber");
  assert.equal(displayName({ name: " ", email: "a@b.c" }), "a@b.c");
});
