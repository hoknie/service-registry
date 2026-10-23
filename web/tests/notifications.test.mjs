import assert from "node:assert/strict";
import { test } from "node:test";

import { bodyOf, deliveryAddress, deliveryQuery, environmentsOf, readDeliveryParams, validEnvironment, validKey } from "../src/lib/notifications.ts";

test("journal filters come from the address", () => {
  const p = readDeliveryParams((n) => ({ subscription: "ops", status: "failed", page: "2" })[n] ?? null);
  assert.deepEqual(p, { subscription: "ops", status: "failed", page: 2 });
  assert.deepEqual(readDeliveryParams((n) => ({ status: "lost", page: "-1" })[n] ?? null), { subscription: "", status: "", page: 1 });
  assert.equal(deliveryAddress(p), "subscription=ops&status=failed&page=2");
  assert.equal(deliveryAddress({ subscription: "", status: "", page: 1 }), "");
  assert.equal(deliveryQuery(p), "subscription=ops&status=failed&limit=25&offset=25");
});

test("a subscription body copies a record and drops empty environments", () => {
  const s = { key: "ops", channel: "webhook", secret_id: "s1", secret: null, event_types: ["deployment.created"], environments: null,
    locale: "ru", disabled: false, signing: null, node_id: "n", inherited: true, created_at: "", updated_at: "" };
  assert.deepEqual(bodyOf(s, { disabled: true }), { channel: "webhook", secret_id: "s1", event_types: ["deployment.created"],
    environments: null, locale: "ru", disabled: true });
  assert.deepEqual(environmentsOf([" Production ", "production", "", "staging"]), ["production", "staging"]);
  assert.equal(environmentsOf([]), null);
});

test("keys and environments follow the server rules", () => {
  assert.ok(validKey("ops-2"));
  assert.ok(!validKey("-ops"));
  assert.ok(validEnvironment("prod.eu-1"));
  assert.ok(!validEnvironment("prod eu"));
});

test("a recovery follows its failure type", async () => {
  const { toggleEvent, PRIMARY_EVENTS } = await import("../src/lib/notifications.ts");
  assert.deepEqual(PRIMARY_EVENTS, ["deployment.created", "deployment.drift", "link.check_failed", "forge.sync_failed"]);
  let events = toggleEvent(["deployment.created"], "deployment.drift");
  events = toggleEvent(events, "deployment.drift_resolved");
  assert.deepEqual(events, ["deployment.created", "deployment.drift", "deployment.drift_resolved"]);
  assert.deepEqual(toggleEvent(events, "deployment.drift"), ["deployment.created"]);
  assert.deepEqual(toggleEvent(events, "deployment.drift_resolved"), ["deployment.created", "deployment.drift"]);
});
