import { describe, expect, it, vi } from "vitest";
import {
  collectEditorTocEntries,
  createEditorTocPreview,
  type EditorTocBlock,
} from "./editorToc";

describe("editor table of contents", () => {
  it("collects unique heading anchors from editor blocks", () => {
    const blocks: EditorTocBlock[] = [
      { pos: 0, type: "paragraph", text: "[toc]" },
      { pos: 7, type: "heading", level: 1, text: "Overview" },
      { pos: 18, type: "heading", level: 2, text: "Install" },
      { pos: 30, type: "heading", level: 2, text: "Install" },
    ];

    expect(collectEditorTocEntries(blocks)).toEqual([
      { depth: 1, id: "overview", pos: 7, text: "Overview" },
      { depth: 2, id: "install", pos: 18, text: "Install" },
      { depth: 2, id: "install-2", pos: 30, text: "Install" },
    ]);
  });

  it("creates accessible controls that scroll to editor headings", () => {
    const host = document.createElement("div");
    const heading = document.createElement("h2");
    heading.dataset.editorHeadingId = "install";
    heading.scrollIntoView = vi.fn();
    host.append(heading);

    const preview = createEditorTocPreview([
      { depth: 2, id: "install", pos: 4, text: "Install" },
    ]);
    host.append(preview);

    preview.querySelector<HTMLButtonElement>("button")?.click();

    expect(heading.scrollIntoView).toHaveBeenCalledWith({
      behavior: "smooth",
      block: "start",
    });
  });
});
