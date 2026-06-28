import { describe, expect, it } from "vitest";
// @ts-ignore The app tsconfig does not include Node types, but Vitest runs this file in Node.
import { readFileSync } from "node:fs";

const cwd = (globalThis as typeof globalThis & { process: { cwd(): string } }).process.cwd();
const css = readFileSync(`${cwd}/src/lib/styles/global.css`, "utf8");

function mediaBlock(query: string): string {
  const start = css.indexOf(query);
  expect(start).toBeGreaterThanOrEqual(0);

  const open = css.indexOf("{", start);
  let depth = 0;
  for (let index = open; index < css.length; index += 1) {
    if (css[index] === "{") depth += 1;
    if (css[index] === "}") depth -= 1;
    if (depth === 0) return css.slice(open + 1, index);
  }

  throw new Error(`Unclosed media block: ${query}`);
}

describe("global sidebar styles", () => {
  it("keeps compact sidebar button rules scoped to sidebar controls", () => {
    expect(css).not.toMatch(/\.sidebar\s+button\s*\{/);
    expect(css).not.toMatch(/\.sidebar\.compact\s+button\s*\{/);
    expect(css).toContain(".sidebar.compact .sidebar__nav > button");
  });

  it("hides the desktop sidebar when the mobile navigation breakpoint is active", () => {
    const mobileLayout = mediaBlock("@media (max-width: 960px)");

    expect(mobileLayout).toContain(".mobile-nav");
    expect(mobileLayout).toContain(".sidebar {\n    display: none;\n  }");
  });
});
