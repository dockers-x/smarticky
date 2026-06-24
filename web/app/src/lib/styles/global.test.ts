import { describe, expect, it } from "vitest";
// @ts-ignore The app tsconfig does not include Node types, but Vitest runs this file in Node.
import { readFileSync } from "node:fs";

const cwd = (globalThis as typeof globalThis & { process: { cwd(): string } }).process.cwd();
const css = readFileSync(`${cwd}/src/lib/styles/global.css`, "utf8");

describe("global sidebar styles", () => {
  it("keeps compact sidebar button rules scoped to sidebar controls", () => {
    expect(css).not.toMatch(/\.sidebar\s+button\s*\{/);
    expect(css).not.toMatch(/\.sidebar\.compact\s+button\s*\{/);
    expect(css).toContain(".sidebar.compact .sidebar__nav > button");
  });
});
