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

describe("renderMarkdown diagram placeholders", () => {
  it("emits a safe Mermaid placeholder", () => {
    const source = "flowchart TD\n  A --> B";
    const html = renderMarkdown(`\`\`\`mermaid\n${source}\n\`\`\``);
    const node = getPlaceholder(html);

    expect(node.dataset.diagramType).toBe("mermaid");
    expect(decodeDiagramSource(node.dataset.diagramSource || "")).toBe(source);
    expect(html).not.toContain("<script");
  });

  it("emits a safe drawio placeholder", () => {
    const source = "<mxfile><diagram>safe</diagram></mxfile>";
    const html = renderMarkdown(`\`\`\`drawio\n${source}\n\`\`\``);
    const node = getPlaceholder(html);

    expect(node.dataset.diagramType).toBe("drawio");
    expect(decodeDiagramSource(node.dataset.diagramSource || "")).toBe(source);
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
  it("does not count Mermaid or drawio source as visible prose", () => {
    const markdown = ["Visible", "```mermaid", "flowchart TD", "A --> B", "```"].join("\n");

    expect(stripMarkdown(markdown)).toBe("Visible");
  });
});
