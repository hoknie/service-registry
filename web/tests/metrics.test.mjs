import assert from "node:assert/strict";
import { test } from "node:test";

import { avg, formatBytes, formatUptime, labelNames, matchesFilter, num, pickFamily, pushPoint, rate, rateSeries, rates, seriesKey, uptimeSeconds } from "../src/lib/metrics.ts";
import { metricsVisible } from "../src/lib/rights.ts";

test("a counter going down is a restart, not a negative rate", () => {
  assert.equal(rate({ t: 0, v: 10 }, { t: 2000, v: 20 }), 5);
  assert.equal(rate({ t: 0, v: 10 }, { t: 2000, v: 3 }), null);
  assert.equal(rate(undefined, { t: 2000, v: 3 }), null);
  assert.equal(rate({ t: 2000, v: 1 }, { t: 2000, v: 3 }), null);
  assert.deepEqual(rates([{ t: 0, v: 0 }, { t: 1000, v: 2 }, { t: 2000, v: 1 }, { t: 3000, v: 4 }]), [
    { t: 1000, v: 2 },
    { t: 3000, v: 3 },
  ]);
});

test("the buffer keeps the last points and replaces a point of the same time", () => {
  let b = [];
  for (let i = 0; i < 130; i++) b = pushPoint(b, { t: i, v: i });
  assert.equal(b.length, 120);
  assert.equal(b[0].t, 10);
  b = pushPoint(b, { t: 129, v: 7 });
  assert.equal(b.length, 120);
  assert.equal(b.at(-1).v, 7);
});

test("bytes use binary units in the locale's format", () => {
  assert.equal(formatBytes(1536, "ru"), "1,5 KiB");
  assert.equal(formatBytes(512, "en"), "512 B");
  assert.equal(formatBytes(3 * 1024 * 1024 * 1024, "en"), "3 GiB");
});

test("averages and numbers handle empty and non-finite values", () => {
  assert.equal(avg(0, 0), null);
  assert.equal(avg(3, 2), 1.5);
  assert.equal(num("+Inf"), Infinity);
  assert.ok(Number.isNaN(num("NaN")));
  assert.equal(num(null), null);
  assert.equal(uptimeSeconds("2026-10-25T10:00:00Z", Date.parse("2026-10-25T10:01:30Z")), 90);
  assert.equal(uptimeSeconds(null, 0), null);
});

test("series keys, label names and filters", () => {
  assert.equal(seriesKey("x", { b: "2", a: "1" }), "x{a=1,b=2}");
  const series = [{ labels: { route: "/api/health", method: "GET" } }, { labels: { job: "sync" } }];
  assert.deepEqual(labelNames(series), ["job", "method", "route"]);
  assert.equal(matchesFilter(series[0], "HEALTH"), true);
  assert.equal(matchesFilter(series[1], "health"), false);
  assert.equal(matchesFilter(series[1], " "), true);
});

test("the chosen family falls back to the default, then to the first", () => {
  const fams = [{ name: "a" }, { name: "svcr_http_requests_total" }];
  assert.equal(pickFamily(fams, "a"), "a");
  assert.equal(pickFamily(fams, "gone"), "svcr_http_requests_total");
  assert.equal(pickFamily([{ name: "a" }], null), "a");
  assert.equal(pickFamily([], null), null);
});

test("the metrics section needs the feature and a settings read right", () => {
  const on = { metrics: true };
  assert.equal(metricsVisible({ is_superadmin: true, features: on }), true);
  assert.equal(metricsVisible({ is_superadmin: true, features: { metrics: false } }), false);
  assert.equal(metricsVisible({ is_superadmin: false, rights: { scans: ["read"] }, features: on }), true);
  assert.equal(metricsVisible({ is_superadmin: false, rights: { secrets: ["read"] }, features: on }), true);
  assert.equal(metricsVisible({ is_superadmin: false, rights: { clusters: ["read"] }, features: on }), false);
  assert.equal(metricsVisible(null), false);
});

test("uptime shows the two largest units", () => {
  assert.equal(formatUptime(42, "en"), "42 sec");
  assert.equal(formatUptime(3 * 86400 + 5 * 3600 + 7 * 60, "en"), "3 days 5 hr");
  assert.equal(formatUptime(3720, "en"), "1 hr 2 min");
});

test("rateSeries shows zero until a second sample", () => {
  assert.deepEqual(rateSeries([]), []);
  assert.deepEqual(rateSeries([{ t: 5000, v: 7 }]), [{ t: 5000, v: 0 }]);
  assert.deepEqual(rateSeries([{ t: 0, v: 1 }, { t: 1000, v: 3 }]), [{ t: 1000, v: 2 }]);
});
