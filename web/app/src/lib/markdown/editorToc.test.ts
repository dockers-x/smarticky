import { describe, expect, it, vi } from "vitest";
import {
  collectEditorTocEntries,
  createEditorTocLayer,
  createEditorTocPreview,
  renderEditorTocLayer,
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

  it("marks editor TOC items with depth hooks for hierarchy styling", () => {
    const preview = createEditorTocPreview([
      { depth: 2, id: "install", pos: 4, text: "Install" },
      { depth: 3, id: "run", pos: 12, text: "Run" },
    ]);

    const items = [...preview.querySelectorAll("li")];
    expect(items.map((item) => item.dataset.tocDepth)).toEqual(["2", "3"]);
    expect(items[0]?.classList.contains("markdown-toc__item--depth-2")).toBe(true);
    expect(items[1]?.classList.contains("markdown-toc__item--depth-3")).toBe(true);
    expect(
      [...preview.querySelectorAll<HTMLButtonElement>("button")].map(
        (button) => button.dataset.tocDepth,
      ),
    ).toEqual(["2", "3"]);
  });

  it("renders the editor TOC in a host layer that can be hidden without document content", () => {
    const layer = createEditorTocLayer();

    expect(layer.className).toBe("editor-toc-layer");
    expect(layer.contentEditable).toBe("false");
    expect(layer.hidden).toBe(true);

    renderEditorTocLayer(
      layer,
      [{ depth: 2, id: "install", pos: 4, text: "Install" }],
      true,
    );

    expect(layer.hidden).toBe(false);
    expect(layer.querySelector("nav")?.classList.contains("editor-toc-panel")).toBe(true);
    expect(layer.querySelector("button")?.dataset.editorHeadingTarget).toBe("install");

    renderEditorTocLayer(layer, [], false);

    expect(layer.hidden).toBe(true);
    expect(layer.childElementCount).toBe(0);
  });
});
