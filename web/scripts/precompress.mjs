import { readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { constants, gzipSync } from "node:zlib";

const root = process.argv[2] ?? "out";
const kinds = /\.(html|js|css|txt|json|svg|map)$/;

let count = 0;
const walk = (dir) => {
  for (const name of readdirSync(dir)) {
    const path = join(dir, name);
    const info = statSync(path);
    if (info.isDirectory()) walk(path);
    else if (kinds.test(name) && info.size > 1024) {
      writeFileSync(`${path}.gz`, gzipSync(readFileSync(path), { level: constants.Z_BEST_COMPRESSION }));
      count++;
    }
  }
};

walk(root);
console.log(`precompress: ${count} files in ${root}`);
