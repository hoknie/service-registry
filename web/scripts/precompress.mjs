import { readdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { constants, gzipSync } from "node:zlib";

const root = process.argv[2] ?? "out";
const kinds = /\.(html|js|css|txt|json|svg|map)$/;

let count = 0;
const walk = (dir) => {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) walk(path);
    else if (entry.isFile() && kinds.test(entry.name)) {
      const body = readFileSync(path);
      if (body.length <= 1024) continue;
      writeFileSync(`${path}.gz`, gzipSync(body, { level: constants.Z_BEST_COMPRESSION }));
      count++;
    }
  }
};

walk(root);
console.log(`precompress: ${count} files in ${root}`);
