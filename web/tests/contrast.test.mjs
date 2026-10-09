import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

import { ratio, themeTokens } from "../src/lib/contrast.ts";

const css = readFileSync(new URL("../src/app/globals.css", import.meta.url), "utf8");
const themes = {
  light: themeTokens(css, ":root {"),
  dark: themeTokens(css, '[data-theme="dark"] {'),
};
const kinds = ["kind-org", "kind-folder", "kind-project"];
const statuses = ["signal", "cobalt", "amber", "danger", ...kinds];
const backgrounds = ["canvas", "surface", "surface-2", "hero-to", ...kinds.map((k) => `${k}-soft`)];

test("dark tokens are the same with data-theme and with the system preference", () => {
  assert.deepEqual(themeTokens(css, ":root:not([data-theme]) {"), themes.dark);
});

for (const [name, t] of Object.entries(themes)) {
  test(`${name}: text on every background is at least 4.5:1`, () => {
    for (const fg of ["ink", "ink-2", "muted"]) {
      for (const bg of backgrounds) {
        const r = ratio(t[fg], t[bg]);
        assert.ok(r >= 4.5, `${fg} on ${bg}: ${r.toFixed(2)}`);
      }
    }
  });

  test(`${name}: accent colors on their soft backgrounds and on surfaces are at least 4.5:1`, () => {
    for (const c of statuses) {
      for (const bg of [`${c}-soft`, "surface", "canvas"]) {
        const r = ratio(t[c], t[bg]);
        assert.ok(r >= 4.5, `${c} on ${bg}: ${r.toFixed(2)}`);
      }
    }
  });

  test(`${name}: field borders are at least 3:1`, () => {
    for (const bg of ["surface", "canvas"]) {
      const r = ratio(t.field, t[bg]);
      assert.ok(r >= 3, `field on ${bg}: ${r.toFixed(2)}`);
    }
  });
}
