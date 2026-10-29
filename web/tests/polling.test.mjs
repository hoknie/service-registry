import assert from "node:assert/strict";
import { test } from "node:test";

import { draftErrors, draftOf, pollingBody } from "../src/lib/polling.ts";

const polling = (own = {}, values = {}) => ({
  interval_secs: null,
  timeout_secs: 30,
  mode: "on",
  ...values,
  own,
  from: { interval_secs: null, timeout_secs: null, mode: null },
  defaults: { repo_sync_interval_secs: 900, docs_interval_secs: 300, skills_interval_secs: 3600, timeout_secs: 30 },
});

test("a draft keeps only the fields set on the node", () => {
  const d = draftOf(polling({ mode: "off" }, { mode: "off", interval_secs: 1800 }), 900);
  assert.deepEqual(d, { interval: { set: false, minutes: "30" }, timeout: { set: false, seconds: "30" }, mode: { set: true, value: "off" } });
  assert.deepEqual(pollingBody(d), { mode: "off" });
  assert.deepEqual(draftOf(polling(), 900).interval.minutes, "15", "the server default fills an unset period");
});

test("the body sends seconds and only valid set fields", () => {
  const d = { interval: { set: true, minutes: "10" }, timeout: { set: true, seconds: "5" }, mode: { set: false, value: "on" } };
  assert.deepEqual(pollingBody(d), { interval_secs: 600, timeout_secs: 5 });
  assert.deepEqual(draftErrors(d), { interval: false, timeout: false });
  assert.deepEqual(pollingBody({ ...d, mode: { set: true, value: "published" } }).mode, "published");
});

test("values out of range are errors and are not sent", () => {
  for (const minutes of ["0", "1441", "1.5", "", "ten"]) {
    const d = { interval: { set: true, minutes }, timeout: { set: false, seconds: "30" }, mode: { set: false, value: "on" } };
    assert.equal(draftErrors(d).interval, true, minutes);
    assert.deepEqual(pollingBody(d), {}, minutes);
  }
  const d = { interval: { set: false, minutes: "x" }, timeout: { set: true, seconds: "601" }, mode: { set: false, value: "on" } };
  assert.deepEqual(draftErrors(d), { interval: false, timeout: true });
});
