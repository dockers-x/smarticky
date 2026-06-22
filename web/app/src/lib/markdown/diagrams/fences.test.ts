import { describe, expect, it } from "vitest";
import { normalizeDiagramType, stripDiagramFences } from "./fences";

describe("normalizeDiagramType", () => {
  it("recognizes first-version diagram fence languages", () => {
    expect(normalizeDiagramType("mermaid")).toBe("mermaid");
    expect(normalizeDiagramType("MERMAID")).toBe("mermaid");
    expect(normalizeDiagramType("flowchart")).toBe("mermaid");
    expect(normalizeDiagramType("sequenceDiagram")).toBe("mermaid");
    expect(normalizeDiagramType("classDiagram")).toBe("mermaid");
    expect(normalizeDiagramType("drawio")).toBe("drawio");
    expect(normalizeDiagramType("draw.io")).toBe("drawio");
  });

  it("does not recognize deferred or unsupported diagram languages", () => {
    expect(normalizeDiagramType("plantuml")).toBeNull();
    expect(normalizeDiagramType("puml")).toBeNull();
    expect(normalizeDiagramType("dot")).toBeNull();
    expect(normalizeDiagramType("")).toBeNull();
    expect(normalizeDiagramType(undefined)).toBeNull();
  });
});

describe("stripDiagramFences", () => {
  it("removes first-version diagram source from plain text extraction", () => {
    const markdown = [
      "Before",
      "```mermaid",
      "flowchart TD",
      "A --> B",
      "```",
      "Middle",
      "```flowchart",
      "A --> C",
      "```",
      "```drawio",
      "<mxfile></mxfile>",
      "```",
      "After",
    ].join("\n");

    expect(stripDiagramFences(markdown)).toBe(["Before", "Middle", "After"].join("\n"));
  });

  it("leaves deferred PlantUML fences in plain text extraction", () => {
    const markdown = ["```plantuml", "@startuml", "A -> B", "@enduml", "```"].join("\n");

    expect(stripDiagramFences(markdown)).toContain("@startuml");
  });
});
