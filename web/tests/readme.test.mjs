import assert from "node:assert/strict";
import { test } from "node:test";

import { forgeBases, renderReadme } from "../src/lib/readme.ts";

const src = { webUrl: "https://github.example/acme/api", branch: "main", kind: "github" };

test("raw HTML is shown as text and never executed", () => {
  const html = renderReadme("<script>alert(1)</script>\n\n**bold** <img src=x onerror=alert(2)>", src);
  assert.ok(!html.includes("<script"), html);
  assert.ok(!html.includes("<img src=\"x\""), html);
  assert.ok(html.includes("&lt;script&gt;alert(1)&lt;/script&gt;"), html);
  assert.ok(html.includes("<strong>bold</strong>"), html);
});

test("dangerous link schemes are dropped", () => {
  for (const md of ["[x](javascript:alert(1))", "[x](data:text/html,hi)", "[x](vbscript:x)", "![x](javascript:alert(1))"]) {
    const html = renderReadme(md, src);
    assert.ok(!/(href|src)="(javascript|data|vbscript):/i.test(html), html);
    assert.ok(!html.includes("<a ") && !html.includes("<img "), html);
  }
});

test("links open in a new tab, relative ones point at the forge", () => {
  const html = renderReadme("[docs](docs/guide.md) [site](https://example.com) [top](#top)\n\n![logo](./img/logo.png)", src);
  assert.ok(html.includes('href="https://github.example/acme/api/blob/main/docs/guide.md"'), html);
  assert.ok(html.includes('href="https://example.com" target="_blank" rel="noopener nofollow"'), html);
  assert.ok(html.includes('href="#top"') && !html.includes('href="#top" target'), html);
  assert.ok(html.includes('src="https://github.example/acme/api/raw/main/img/logo.png"'), html);
});

test("bases follow the forge", () => {
  assert.equal(forgeBases({ webUrl: "https://gl.example/g/p", branch: "release/1", kind: "gitlab" }).raw, "https://gl.example/g/p/-/raw/release/1/");
  assert.equal(forgeBases({ webUrl: "https://code.example/o/r/", branch: null, kind: "gitea" }).blob, "https://code.example/o/r/src/branch/HEAD/");
});

test("a link to a collected file opens it in the app", () => {
  const files = new Set(["docs/api.md", "README.md"]);
  const docLink = (p) => (files.has(p) ? `/en/catalog?node=n&tab=docs&doc=${encodeURIComponent(p)}` : null);
  const html = renderReadme("[API](api.md) [root](../README.md#top) [other](other.md) [site](https://example.com)", {
    branch: "local",
    dir: "docs",
    docLink,
  });
  assert.ok(html.includes('<a href="/en/catalog?node=n&amp;tab=docs&amp;doc=docs%2Fapi.md">API</a>'), html);
  assert.ok(html.includes('doc=README.md">root</a>'), html);
  assert.ok(!html.includes('href="other.md"') && html.includes("other"), html);
  assert.ok(html.includes('href="https://example.com" target="_blank"'), html);
});

test("without a forge relative images show their alt text", () => {
  const html = renderReadme("![the logo](img/logo.png) ![remote](https://example.com/x.png)", { branch: "local" });
  assert.ok(!html.includes('src="img/logo.png"') && html.includes("the logo"), html);
  assert.ok(html.includes('src="https://example.com/x.png"'), html);
});

test("with a forge, links to files outside the snapshot still go to the forge", () => {
  const html = renderReadme("[x](x.md) [a](a.md)", { ...src, dir: "docs", docLink: (p) => (p === "docs/a.md" ? "/in-app" : null) });
  assert.ok(html.includes('href="https://github.example/acme/api/blob/main/docs/x.md"'), html);
  assert.ok(html.includes('<a href="/in-app">a</a>'), html);
});
