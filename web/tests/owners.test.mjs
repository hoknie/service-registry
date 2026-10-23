import assert from "node:assert/strict";
import { test } from "node:test";

import { badgeNames, contactLink, contactText, draftOf, moveItem, ownerFilter, ownersBody, sameSubject, webLink } from "../src/lib/owners.ts";

test("contact links use only safe schemes", () => {
  assert.deepEqual(contactLink({ kind: "email", value: "team@example.com", label: null }), { href: "mailto:team@example.com", external: false });
  assert.deepEqual(contactLink({ kind: "phone", value: "+7 999 123-45-67", label: null }), { href: "tel:+79991234567", external: false });
  assert.deepEqual(contactLink({ kind: "chat", value: "https://chat.example/p", label: "#p" }), { href: "https://chat.example/p", external: true });
  assert.equal(contactLink({ kind: "url", value: "javascript:alert(1)", label: null }), null);
  assert.equal(webLink("https://oncall.example"), "https://oncall.example");
  assert.equal(webLink("http://oncall.example"), null);
  assert.equal(webLink(null), null);
  assert.equal(contactText({ kind: "chat", value: "https://chat.example/p", label: "#p" }), "#p");
  assert.equal(contactText({ kind: "email", value: "a@example.com", label: null }), "a@example.com");
});

test("drafts become the request body", () => {
  const group = { subject_kind: "group", subject_id: "g", name: "Platform", email: null, status: null, contacts: [], on_call_url: null, node_id: "n", inherited: true };
  const user = { ...group, subject_kind: "user", name: "Bob", email: "bob@example.com", status: "active" };
  const draft = [draftOf(group), draftOf(user)];
  assert.deepEqual(ownersBody(draft), {
    owners: [
      { subject_kind: "group", subject: "Platform" },
      { subject_kind: "user", subject: "bob@example.com" },
    ],
  });
  assert.equal(sameSubject(draft[0], { ...draft[0], subject: "platform" }), true);
  assert.equal(sameSubject(draft[0], draft[1]), false);
});

test("order moves and badges", () => {
  assert.deepEqual(moveItem(["a", "b", "c"], 2, -1), ["a", "c", "b"]);
  assert.deepEqual(moveItem(["a", "b"], 0, -1), ["a", "b"]);
  assert.deepEqual(badgeNames([{ name: "A" }, { name: "B" }, { name: "C" }]), { names: ["A", "B"], more: 1 });
  assert.deepEqual(badgeNames([]), { names: [], more: 0 });
  assert.equal(ownerFilter("group", "x"), "group:x");
});
