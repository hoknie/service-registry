import { existsSync, readFileSync, readdirSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const web = fileURLToPath(new URL("..", import.meta.url));
const out = join(web, "out");
const LOCALES = ["en", "es", "ru", "zh"];
const PAGES = ["", "login", "account", "catalog", "catalog/new", "search", "admin", "admin/users", "admin/users/user", "admin/groups", "admin/groups/group", "admin/tokens", "admin/link-kinds", "admin/clusters", "admin/environments", "admin/scans", "admin/secrets", "doc", "404"];

const problems = [];

if (!existsSync(out)) {
  console.error(`check-export: ${relative(web, out)}/ is missing — run \`pnpm build\` first`);
  process.exit(1);
}

function read(rel) {
  try {
    return readFileSync(join(out, rel), "utf8");
  } catch {
    problems.push(`${rel}: missing`);
    return undefined;
  }
}

for (const locale of LOCALES) {
  for (const page of PAGES) {
    const rel = page ? `${locale}/${page}.html` : `${locale}.html`;
    const html = read(rel);
    if (html !== undefined && !html.includes(`<html lang="${locale}"`)) {
      problems.push(`${rel}: no <html lang="${locale}">`);
    }
  }
}
read("404.html");

const INLINE_HANDLER = /<[a-z][^>]*?\son[a-z]+\s*=/i;
const JAVASCRIPT_URL = /<[a-z][^>]*?\s(?:href|src|action|formaction)\s*=\s*["']?\s*javascript:/i;

function files(dir) {
  if (!existsSync(dir)) return [];
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory() ? files(join(dir, e.name)) : [join(dir, e.name)],
  );
}

for (const file of files(out).filter((f) => f.endsWith(".html"))) {
  const markup = readFileSync(file, "utf8").replace(/<script\b[^>]*>[\s\S]*?<\/script[^>]*>/gi, "<script></script>");
  if (INLINE_HANDLER.test(markup)) problems.push(`${relative(out, file)}: inline event handler (not allowed by CSP)`);
  if (JAVASCRIPT_URL.test(markup)) problems.push(`${relative(out, file)}: javascript: URL (not allowed by CSP)`);
}

const js = files(join(out, "_next", "static")).filter((f) => f.endsWith(".js"));
if (js.length === 0) problems.push("_next/static/: no JS assets");

function leaves(value) {
  return typeof value === "string" ? [value] : Object.values(value).flatMap(leaves);
}
const dict = (locale) =>
  JSON.parse(readFileSync(join(web, "src", "i18n", "messages", `${locale}.json`), "utf8"));
const english = new Set(leaves(dict("en")));
const markers = LOCALES.filter((l) => l !== "en").flatMap((locale) =>
  leaves(dict(locale))
    .filter((text) => text.length >= 12 && !english.has(text))
    .map((text) => ({ locale, text })),
);
const escaped = (text) =>
  text.replace(/[^\x20-\x7e]/g, (c) => `\\u${c.charCodeAt(0).toString(16).padStart(4, "0")}`);

for (const file of js) {
  const source = readFileSync(file, "utf8");
  const lower = source.toLowerCase();
  const hit = markers.find(
    ({ text }) => source.includes(text) || lower.includes(escaped(text).toLowerCase()),
  );
  if (hit) {
    problems.push(`${relative(out, file)}: contains the ${hit.locale} dictionary ("${hit.text}")`);
  }
}

if (problems.length > 0) {
  console.error(`check-export: ${problems.length} problem(s) in ${relative(web, out)}/`);
  for (const p of problems) console.error(`  - ${p}`);
  process.exit(1);
}
console.log(
  `check-export: ok — ${LOCALES.length} locales × ${PAGES.length} pages, 404.html, ` +
    `${js.length} JS assets without es/ru/zh dictionaries (${markers.length} markers)`,
);
