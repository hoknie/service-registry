import assert from "node:assert/strict";
import { test } from "node:test";

import { hostingBases, renderReadme } from "../src/lib/readme.ts";

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

test("links open in a new tab, relative ones point at the hosting", () => {
  const html = renderReadme("[docs](docs/guide.md) [site](https://example.com) [top](#top)\n\n![logo](./img/logo.png)", src);
  assert.ok(html.includes('href="https://github.example/acme/api/blob/main/docs/guide.md"'), html);
  assert.ok(html.includes('href="https://example.com" target="_blank" rel="noopener nofollow"'), html);
  assert.ok(html.includes('href="#top"') && !html.includes('href="#top" target'), html);
  assert.ok(html.includes('src="https://github.example/acme/api/raw/main/img/logo.png"'), html);
});

test("bases follow the hosting", () => {
  assert.equal(hostingBases({ webUrl: "https://gl.example/g/p", branch: "release/1", kind: "gitlab" }).raw, "https://gl.example/g/p/-/raw/release/1/");
  assert.equal(hostingBases({ webUrl: "https://code.example/o/r/", branch: null, kind: "gitea" }).blob, "https://code.example/o/r/src/branch/HEAD/");
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

test("without a hosting relative images show their alt text", () => {
  const html = renderReadme("![the logo](img/logo.png) ![remote](https://example.com/x.png)", { branch: "local" });
  assert.ok(!html.includes('src="img/logo.png"') && html.includes("the logo"), html);
  assert.ok(html.includes('src="https://example.com/x.png"'), html);
});

test("with a hosting, links to files outside the snapshot still go to the hosting", () => {
  const html = renderReadme("[x](x.md) [a](a.md)", { ...src, dir: "docs", docLink: (p) => (p === "docs/a.md" ? "/in-app" : null) });
  assert.ok(html.includes('href="https://github.example/acme/api/blob/main/docs/x.md"'), html);
  assert.ok(html.includes('<a href="/in-app">a</a>'), html);
});

test("task list items render as disabled checkboxes without the marker", () => {
  const html = renderReadme("- [x] Готово\n- [ ] Сделать\n- обычный", src);
  assert.ok(html.includes('<ul class="contains-task-list">'), html);
  assert.ok(html.includes('<li class="task-list-item"><input type="checkbox" class="task-list-item-checkbox" disabled aria-label="Готово" checked> Готово</li>'), html);
  assert.ok(html.includes('aria-label="Сделать"> Сделать</li>'), html);
  assert.ok(!html.includes("[x]") && !html.includes("[ ]"), html);
  assert.ok(html.includes("<li>обычный</li>"), html);
});

test("checkbox HTML in the source stays text", () => {
  const html = renderReadme('<input type="checkbox" checked>\n\n- [x] <b>x</b>', src);
  assert.ok(html.includes("&lt;input type=&quot;checkbox&quot; checked&gt;"), html);
  assert.ok(html.includes("&lt;b&gt;x&lt;/b&gt;"), html);
  assert.equal((html.match(/<input /g) ?? []).length, 1, html);
});

test("headings get anchors and links to them stay in the page", () => {
  const html = renderReadme("## Требования к API\n\n## Требования к API\n\n[см.](#требования-к-api)", src);
  assert.ok(html.includes('<h2 id="требования-к-api">'), html);
  assert.ok(html.includes('<h2 id="требования-к-api-1">'), html);
  assert.ok(/href="#[^"]+"(?! target)/.test(html), html);
});

test("GitHub alerts become titled blocks, unknown markers stay quotes", () => {
  const html = renderReadme("> [!WARNING]\n> Не удалять\n\n> [!DANGER]\n> x", { ...src, alerts: { warning: "Warning" } });
  assert.ok(html.includes('<blockquote class="markdown-alert markdown-alert-warning">'), html);
  assert.ok(html.includes('<p class="markdown-alert-title">Warning</p>'), html);
  assert.ok(html.includes("Не удалять"), html);
  assert.ok(!html.includes("[!WARNING]"), html);
  assert.ok(html.includes("[!DANGER]"), html);
  const only = renderReadme("> [!NOTE]", { ...src, alerts: { note: "Note" } });
  assert.ok(only.includes('<p class="markdown-alert-title">Note</p>') && !only.includes("[!NOTE]"), only);
});

test("tables are wrapped for horizontal scrolling and strikethrough works", () => {
  const html = renderReadme("| a | b |\n|:--|--:|\n| 1 | 2 |\n\n~~old~~", src);
  assert.ok(html.includes('<div class="table-wrap"><table>'), html);
  assert.ok(html.includes('style="text-align:right"'), html);
  assert.ok(html.includes("<s>old</s>"), html);
});

test("headings get a # link to themselves when a label is given", () => {
  const html = renderReadme("## Требования к API", { ...src, anchorLabel: "Link to this section" });
  assert.ok(
    html.includes('<h2 id="требования-к-api">Требования к API <a class="heading-anchor" href="#%D1%82%D1%80%D0%B5%D0%B1%D0%BE%D0%B2%D0%B0%D0%BD%D0%B8%D1%8F-%D0%BA-api" aria-label="Link to this section">#</a></h2>'),
    html,
  );
  assert.ok(!renderReadme("## A", src).includes("heading-anchor"));
});
