import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

import { cssComments, isDirective, scriptComments } from "./comments.mjs";

const root = new URL("..", import.meta.url).pathname;
const files = [];
const walk = (dir) => {
  for (const name of readdirSync(dir)) {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) walk(path);
    else if (/\.(ts|tsx|mjs|css)$/.test(name) && !name.endsWith(".d.ts")) files.push(path);
  }
};
for (const dir of ["src", "scripts", "tests"]) walk(join(root, dir));

const lineOf = (text, pos) => text.slice(0, pos).split("\n").length;
let bad = 0;
for (const file of files) {
  const text = readFileSync(file, "utf8");
  const found = file.endsWith(".css") ? cssComments(text) : scriptComments(file, text).comments;
  for (const c of found) {
    if (isDirective(c.text)) continue;
    bad++;
    console.error(`${relative(root, file)}:${lineOf(text, c.pos)}: comment ${JSON.stringify(c.text.split("\n")[0].slice(0, 60))}`);
  }
}
if (bad) {
  console.error(`check-comments: ${bad} comment(s) outside directives (ADR-0051)`);
  process.exit(1);
}
console.log(`check-comments: ok — ${files.length} files without comments`);
