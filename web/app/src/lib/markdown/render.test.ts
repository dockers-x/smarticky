import { describe, expect, it } from "vitest";
import { decodeDiagramSource } from "./diagrams/placeholders";
import { renderMarkdown, stripMarkdown } from "./render";

function getPlaceholder(html: string): HTMLElement {
  const root = document.createElement("div");
  root.innerHTML = html;
  const node = root.querySelector<HTMLElement>("[data-diagram-placeholder='true']");
  if (!node) throw new Error("diagram placeholder missing");
  return node;
}

function getWhiteboardReference(html: string): HTMLElement {
  const root = document.createElement("div");
  root.innerHTML = html;
  const node = root.querySelector<HTMLElement>("[data-whiteboard-reference='true']");
  if (!node) throw new Error("whiteboard reference missing");
  return node;
}

const whiteboardID = "123e4567-e89b-12d3-a456-426614174000";

describe("renderMarkdown diagram placeholders", () => {
  it("emits a safe Mermaid placeholder", () => {
    const source = "flowchart TD\n  A --> B";
    const html = renderMarkdown(`\`\`\`mermaid\n${source}\n\`\`\``);
    const node = getPlaceholder(html);

    expect(node.dataset.diagramType).toBe("mermaid");
    expect(decodeDiagramSource(node.dataset.diagramSource || "")).toBe(source);
    expect(html).not.toContain("<script");
  });

  it("emits a Mermaid placeholder for selected Mermaid diagram types", () => {
    const html = renderMarkdown("```flowchart\nA->B\nB->C\n```");
    const node = getPlaceholder(html);

    expect(node.dataset.diagramType).toBe("mermaid");
    expect(decodeDiagramSource(node.dataset.diagramSource || "")).toBe(
      "flowchart TD\nA-->B\nB-->C",
    );
  });

  it("emits a safe drawio placeholder", () => {
    const source = "<mxfile><diagram>safe</diagram></mxfile>";
    const html = renderMarkdown(`\`\`\`drawio\n${source}\n\`\`\``);
    const node = getPlaceholder(html);

    expect(node.dataset.diagramType).toBe("drawio");
    expect(decodeDiagramSource(node.dataset.diagramSource || "")).toBe(source);
  });

  it("accepts drawio XML copied from a .drawio file", () => {
    const source = [
      "<mxfile>",
      '  <diagram name="Page-1">',
      "    <mxGraphModel><root><mxCell id=\"0\"/></root></mxGraphModel>",
      "  </diagram>",
      "</mxfile>",
    ].join("\n");
    const html = renderMarkdown(`\`\`\`drawio\n${source}\n\`\`\``);
    const node = getPlaceholder(html);

    expect(node.dataset.diagramType).toBe("drawio");
    expect(decodeDiagramSource(node.dataset.diagramSource || "")).toBe(source);
  });

  it("emits a safe Excalidraw whiteboard reference", () => {
    const html = renderMarkdown(
      `\`\`\`excalidraw\nwhiteboard: ${whiteboardID}\n\`\`\``,
    );
    const node = getWhiteboardReference(html);

    expect(node.dataset.whiteboardId).toBe(whiteboardID);
    expect(html).not.toContain("<script");
  });

  it("keeps unsupported diagram fences as normal code blocks", () => {
    const html = renderMarkdown("```plantuml\n@startuml\nA -> B\n@enduml\n```");

    expect(html).toContain("<pre>");
    expect(html).toContain("@startuml");
    expect(html).not.toContain("data-diagram-placeholder");
  });

  it("keeps empty supported fences as normal code blocks", () => {
    const html = renderMarkdown("```mermaid\n\n```");

    expect(html).toContain("<pre>");
    expect(html).not.toContain("data-diagram-placeholder");
  });
});

describe("stripMarkdown", () => {
  it("does not count Mermaid, drawio, or whiteboard references as visible prose", () => {
    const markdown = [
      "Visible",
      "```mermaid",
      "flowchart TD",
      "A --> B",
      "```",
      "```excalidraw",
      `whiteboard: ${whiteboardID}`,
      "```",
    ].join("\n");

    expect(stripMarkdown(markdown)).toBe("Visible");
  });
});
