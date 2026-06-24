import { describe, expect, it, vi } from "vitest";
import {
  createEditorDiagramCodeBlockConfig,
  diagramCodeLanguages,
} from "./editorPreview";

function flushMicrotasks(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

describe("editor diagram preview", () => {
  it("registers Mermaid and drawio languages for code fences", () => {
    expect(diagramCodeLanguages.map((language) => language.name)).toEqual(
      expect.arrayContaining([
        "mermaid",
        "flowchart",
        "sequenceDiagram",
        "classDiagram",
        "packet",
        "drawio",
        "excalidraw",
      ]),
    );
    expect(
      diagramCodeLanguages.find((language) => language.name === "drawio")
        ?.alias,
    ).toContain("draw.io");
    for (const language of diagramCodeLanguages) {
      expect(language.alias).toContain(language.name.toLowerCase());
    }
  });

  it("keeps default programming language highlighting when adding diagram languages", () => {
    const config = createEditorDiagramCodeBlockConfig({
      getTheme: () => "light",
    });

    expect(config.languages?.map((language) => language.name)).toEqual(
      expect.arrayContaining(["JavaScript", "Go", "mermaid", "drawio"]),
    );
  });

  it("does not render unsupported code block languages", () => {
    const render = vi.fn();
    const applyPreview = vi.fn();
    const config = createEditorDiagramCodeBlockConfig({
      getTheme: () => "light",
      render,
    });

    const result = config.renderPreview?.(
      "javascript",
      "console.log(1)",
      applyPreview,
    );

    expect(result).toBeNull();
    expect(render).not.toHaveBeenCalled();
    expect(applyPreview).not.toHaveBeenCalled();
  });

  it("renders Mermaid previews asynchronously with the active theme", async () => {
    const render = vi.fn().mockResolvedValue({
      html: `<div class="diagram-render">ok</div>`,
    });
    const applyPreview = vi.fn();
    const config = createEditorDiagramCodeBlockConfig({
      getTheme: () => "dark",
      render,
    });

    const result = config.renderPreview?.(
      "mermaid",
      " flowchart TD\nA --> B ",
      applyPreview,
    );
    await flushMicrotasks();

    expect(result).toBeUndefined();
    expect(applyPreview).toHaveBeenCalledWith(
      expect.stringContaining("diagram-loading"),
    );
    expect(render).toHaveBeenCalledWith({
      type: "mermaid",
      source: "flowchart TD\nA --> B",
      theme: "dark",
    });
    expect(applyPreview).toHaveBeenLastCalledWith(
      `<div class="diagram-render">ok</div>`,
    );
  });

  it("renders selected Mermaid type previews with generated declarations", async () => {
    const render = vi.fn().mockResolvedValue({
      html: `<div class="diagram-render">ok</div>`,
    });
    const applyPreview = vi.fn();
    const config = createEditorDiagramCodeBlockConfig({
      getTheme: () => "light",
      render,
    });

    config.renderPreview?.("flowchart", "A->B\nB->C", applyPreview);
    await flushMicrotasks();

    expect(config.renderLanguage?.("flowchart", false)).toBe("Flowchart");
    expect(render).toHaveBeenCalledWith({
      type: "mermaid",
      source: "flowchart TD\nA-->B\nB-->C",
      theme: "light",
    });
  });

  it("renders escaped inline errors for failed drawio previews", async () => {
    const render = vi.fn().mockRejectedValue(new Error("<bad xml>"));
    const applyPreview = vi.fn();
    const config = createEditorDiagramCodeBlockConfig({
      getTheme: () => "light",
      render,
    });

    config.renderPreview?.("draw.io", "<mxfile>", applyPreview);
    await flushMicrotasks();

    expect(applyPreview).toHaveBeenLastCalledWith(
      expect.stringContaining("Failed to render drawio diagram: &lt;bad xml&gt;"),
    );
    expect(applyPreview).toHaveBeenLastCalledWith(
      expect.stringContaining("diagram-error"),
    );
  });

  it("renders Excalidraw whiteboard references without calling diagram renderers", () => {
    const render = vi.fn();
    const applyPreview = vi.fn();
    const config = createEditorDiagramCodeBlockConfig({
      getTheme: () => "light",
      render,
    });

    const html = config.renderPreview?.(
      "excalidraw",
      "whiteboard: 123e4567-e89b-12d3-a456-426614174000",
      applyPreview,
    );

    expect(config.renderLanguage?.("excalidraw", false)).toBe("Excalidraw");
    expect(html).toContain("data-whiteboard-reference");
    expect(render).not.toHaveBeenCalled();
    expect(applyPreview).not.toHaveBeenCalled();
  });
});
