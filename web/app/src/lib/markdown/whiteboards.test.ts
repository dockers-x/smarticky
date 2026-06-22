import { describe, expect, it } from "vitest";
import {
  createWhiteboardReferenceFence,
  extractWhiteboardID,
  isWhiteboardFenceLanguage,
  removeWhiteboardReferenceFences,
} from "./whiteboards";

const whiteboardID = "123e4567-e89b-12d3-a456-426614174000";
const otherWhiteboardID = "123e4567-e89b-12d3-a456-426614174001";

describe("whiteboard markdown references", () => {
  it("recognizes excalidraw fences", () => {
    expect(isWhiteboardFenceLanguage("excalidraw")).toBe(true);
    expect(isWhiteboardFenceLanguage("Excalidraw")).toBe(true);
    expect(isWhiteboardFenceLanguage("mermaid")).toBe(false);
  });

  it("extracts whiteboard IDs from supported reference formats", () => {
    expect(extractWhiteboardID(`whiteboard: ${whiteboardID}`)).toBe(whiteboardID);
    expect(extractWhiteboardID(`id: ${whiteboardID}`)).toBe(whiteboardID);
    expect(extractWhiteboardID(whiteboardID)).toBe(whiteboardID);
  });

  it("creates the canonical fenced reference", () => {
    expect(createWhiteboardReferenceFence(whiteboardID)).toBe(
      ["```excalidraw", `whiteboard: ${whiteboardID}`, "```"].join("\n"),
    );
  });

  it("removes the matching Excalidraw reference fence", () => {
    const markdown = [
      "before",
      "",
      "```excalidraw",
      `whiteboard: ${whiteboardID}`,
      "```",
      "",
      "after",
    ].join("\n");

    expect(removeWhiteboardReferenceFences(markdown, whiteboardID)).toEqual({
      markdown: ["before", "", "after"].join("\n"),
      removedCount: 1,
    });
  });

  it("keeps non-matching whiteboard references and ordinary code fences", () => {
    const markdown = [
      "```excalidraw",
      `whiteboard: ${otherWhiteboardID}`,
      "```",
      "",
      "```ts",
      `const id = "${whiteboardID}";`,
      "```",
    ].join("\n");

    expect(removeWhiteboardReferenceFences(markdown, whiteboardID)).toEqual({
      markdown,
      removedCount: 0,
    });
  });

  it("removes matching tilde fences with id aliases", () => {
    const markdown = [
      "~~~ Excalidraw",
      `id: ${whiteboardID}`,
      "~~~",
      "",
      "body",
    ].join("\n");

    expect(removeWhiteboardReferenceFences(markdown, whiteboardID)).toEqual({
      markdown: "body",
      removedCount: 1,
    });
  });
});
