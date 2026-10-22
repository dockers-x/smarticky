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

describe("paper workspace accessibility", () => {
  it("bounds the workspace to the available visual viewport", () => {
    expect(css).toContain("height: var(--app-height)");
    const mobileLayout = mediaBlock("@media (max-width: 960px)");
    expect(mobileLayout).toContain("overflow: hidden");
    expect(mobileLayout).toContain("min-height: 0");
    expect(css).not.toContain("min-height: 100dvh");
  });

  it("uses readable paper typography while preserving custom fonts", () => {
    expect(css).toContain("--crepe-font-default: var(--editor-font-family, var(--font-reader))");
    expect(css).toContain("font-family: var(--editor-font-family, var(--font-reader))");
  });

  it("provides touch targets and an instant keyboard interaction mode", () => {
    expect(css).toContain("@media (pointer: coarse), (max-width: 960px)");
    expect(css).toContain("min-height: var(--size-touch)");
    expect(css).toContain(".share-dialog__controls button,");
    expect(css).toContain(':root[data-input-method="keyboard"]');
    expect(css).toContain("--duration-sheet: 0ms");
  });

  it("keeps confirmation dialogs inside short viewports", () => {
    expect(css).toContain("max-height: calc(var(--app-height) - 40px)");
    expect(css).toContain("@media (prefers-reduced-transparency: reduce)");
    expect(css).toContain("@media (prefers-contrast: more)");
  });

  it("does not let the mobile writing surface outgrow a keyboard-reduced viewport", () => {
    const mobileLayout = mediaBlock("@media (max-width: 768px)");
    expect(mobileLayout).not.toContain("min-height: calc(100dvh - 90px)");
    expect(mobileLayout).toContain("min-height: max(12rem, calc(var(--app-height) - 220px))");
    expect(css).toContain("height: calc(52px + env(safe-area-inset-top))");
  });
});
