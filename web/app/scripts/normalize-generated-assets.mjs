import { readdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const scriptsDir = dirname(fileURLToPath(import.meta.url));
const assetsDir = join(scriptsDir, "..", "..", "static", "app", "assets");

const svelteWhitespaceSpread =
  "[...` " + "\t" + "\n" + "\\r\\f" + "\u00a0" + "\\v\\uFEFF`]";
const escapedWhitespaceSpread = '[..." \\t\\n\\r\\f\\u00A0\\v\\uFEFF"]';
const cspUnsafeGlobalProbes = [
  'Function("return this")()',
  "Function('return this')()",
];

let changed = 0;

for (const filename of readdirSync(assetsDir)) {
  if (!filename.endsWith(".js")) continue;

  const path = join(assetsDir, filename);
  const source = readFileSync(path, "utf8");
  let normalized = source
    .replaceAll(svelteWhitespaceSpread, escapedWhitespaceSpread)
    .replace(/[ \t]+$/gm, "");
  for (const probe of cspUnsafeGlobalProbes) {
    normalized = normalized.replaceAll(probe, "globalThis");
  }

  if (normalized !== source) {
    writeFileSync(path, normalized);
    changed += 1;
  }
}

if (changed > 0) {
  console.log(`Normalized ${changed} generated JavaScript asset(s).`);
}
