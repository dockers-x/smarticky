import { describe, expect, it } from "vitest";
import {
  createMermaidDiagramFence,
  findMermaidDiagramVariant,
  hasMermaidDiagramDeclaration,
  materializeMermaidSource,
  normalizeFlowchartShorthand,
} from "./mermaidSyntax";

describe("mermaid syntax helpers", () => {
  it("detects declared Mermaid diagram types", () => {
    expect(hasMermaidDiagramDeclaration("sequenceDiagram\nA->>B: hi")).toBe(true);
    expect(hasMermaidDiagramDeclaration("classDiagram\nAnimal <|-- Duck")).toBe(true);
    expect(hasMermaidDiagramDeclaration("A->B")).toBe(false);
  });

  it("materializes selected Mermaid type languages", () => {
    expect(materializeMermaidSource("sequenceDiagram", "A->>B: hi")).toBe(
      "sequenceDiagram\nA->>B: hi",
    );
    expect(materializeMermaidSource("classDiagram", "Animal <|-- Duck")).toBe(
      "classDiagram\nAnimal <|-- Duck",
    );
    expect(materializeMermaidSource("packet-beta", '0-15: "Source Port"')).toBe(
      'packet\n0-15: "Source Port"',
    );
  });

  it("normalizes common flowchart arrow shorthand without changing sequence arrows", () => {
    expect(normalizeFlowchartShorthand("A->B\nB-->C\nC->>D")).toBe(
      "A-->B\nB-->C\nC->>D",
    );
    expect(materializeMermaidSource("flowchart", "A->B")).toBe(
      "flowchart TD\nA-->B",
    );
  });

  it("creates Mermaid fences from typed templates", () => {
    const variant = findMermaidDiagramVariant("sequenceDiagram");

    expect(variant).not.toBeNull();
    expect(createMermaidDiagramFence(variant!)).toContain("```mermaid\nsequenceDiagram");
  });
});
