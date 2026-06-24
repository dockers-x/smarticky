import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderMermaidDiagram } from "./mermaid";

const mermaidMock = vi.hoisted(() => ({
  initialize: vi.fn(),
  parse: vi.fn(),
  render: vi.fn(),
}));

vi.mock("mermaid", () => ({
  default: mermaidMock,
}));

describe("renderMermaidDiagram", () => {
  beforeEach(() => {
    mermaidMock.initialize.mockReset();
    mermaidMock.parse.mockReset().mockResolvedValue(true);
    mermaidMock.render.mockReset().mockResolvedValue({
      svg: "<svg><g><text>diagram</text></g></svg>",
    });
  });

  it("renders Mermaid source as sanitized SVG HTML", async () => {
    const result = await renderMermaidDiagram({
      type: "mermaid",
      source: "flowchart TD\nA --> B",
      theme: "light",
    });

    expect(mermaidMock.initialize).toHaveBeenCalledWith(
      expect.objectContaining({
          startOnLoad: false,
          securityLevel: "loose",
          theme: "base",
        flowchart: expect.objectContaining({
          htmlLabels: true,
        }),
        themeVariables: expect.objectContaining({
          actorTextColor: "#27231f",
          primaryColor: "#fff8f0",
          primaryTextColor: "#27231f",
          signalTextColor: "#27231f",
        }),
      }),
    );
    expect(mermaidMock.parse).toHaveBeenCalledWith("flowchart TD\nA --> B");
    expect(mermaidMock.render).toHaveBeenCalled();
    expect(result.html).toContain("diagram-render diagram-render--mermaid");
    expect(result.html).toContain("<svg");
    expect(result.html).toContain("diagram");
  });

  it("treats bare flowchart edges as a top-down Mermaid flowchart", async () => {
    mermaidMock.parse
      .mockRejectedValueOnce(new Error("No diagram type detected"))
      .mockResolvedValueOnce(true);

    await renderMermaidDiagram({
      type: "mermaid",
      source: "A-->B\nB-->C",
      theme: "light",
    });

    expect(mermaidMock.parse).toHaveBeenNthCalledWith(1, "A-->B\nB-->C");
    expect(mermaidMock.parse).toHaveBeenNthCalledWith(
      2,
      "flowchart TD\nA-->B\nB-->C",
    );
    expect(mermaidMock.render).toHaveBeenCalledWith(
      expect.stringMatching(/^smarticky-mermaid-/),
      "flowchart TD\nA-->B\nB-->C",
    );
  });

  it("keeps declared Mermaid diagram types unchanged", async () => {
    await renderMermaidDiagram({
      type: "mermaid",
      source: "classDiagram\nAnimal <|-- Duck",
      theme: "light",
    });

    expect(mermaidMock.parse).toHaveBeenCalledWith(
      "classDiagram\nAnimal <|-- Duck",
    );
    expect(mermaidMock.render).toHaveBeenCalledWith(
      expect.stringMatching(/^smarticky-mermaid-/),
      "classDiagram\nAnimal <|-- Duck",
    );
  });

  it("does not hide Mermaid syntax errors after the fallback attempt", async () => {
    mermaidMock.parse
      .mockRejectedValueOnce(new Error("No diagram type detected"))
      .mockRejectedValueOnce(new Error("Parse error on line 2"));

    await expect(
      renderMermaidDiagram({
        type: "mermaid",
        source: "not a valid diagram",
        theme: "light",
      }),
    ).rejects.toThrow("Parse error on line 2");

    expect(mermaidMock.render).not.toHaveBeenCalled();
  });

  it("uses high-contrast Mermaid colors for dark diagram theme", async () => {
    await renderMermaidDiagram({
      type: "mermaid",
      source: "sequenceDiagram\nA->>B: hi",
      theme: "dark",
    });

    expect(mermaidMock.initialize).toHaveBeenCalledWith(
      expect.objectContaining({
        theme: "base",
        themeVariables: expect.objectContaining({
          actorTextColor: "#f6f1e8",
          primaryColor: "#26231d",
          primaryTextColor: "#f6f1e8",
          signalTextColor: "#f6f1e8",
        }),
      }),
    );
  });

  it("removes script tags from Mermaid output", async () => {
    mermaidMock.render.mockResolvedValue({
      svg: "<svg><script>alert(1)</script><text>safe</text></svg>",
    });

    const result = await renderMermaidDiagram({
      type: "mermaid",
      source: "flowchart TD\nA --> B",
      theme: "light",
    });

    expect(result.html).not.toContain("<script");
    expect(result.html).toContain("safe");
  });

  it("keeps Mermaid HTML labels visible after sanitizing SVG output", async () => {
    mermaidMock.render.mockResolvedValue({
      svg: [
        '<svg><g class="node"><g class="label" transform="translate(-16, -12)">',
        '<foreignobject width="32" height="24">',
        '<div xmlns="http://www.w3.org/1999/xhtml"><span class="nodeLabel">',
        "<p>灵感</p><script>alert(1)</script>",
        "</span></div></foreignobject></g></g></svg>",
      ].join(""),
    });

    const result = await renderMermaidDiagram({
      type: "mermaid",
      source: "flowchart TD\nA[灵感] --> B",
      theme: "light",
    });

    expect(result.html).toContain("灵感");
    expect(result.html).toContain("text-anchor=\"middle\"");
    expect(result.html).not.toContain("<foreignObject");
    expect(result.html).not.toContain("<script");
    expect(result.html).not.toContain("alert(1)");
  });

  it("restores empty Mermaid flowchart node labels from source", async () => {
    mermaidMock.render.mockResolvedValue({
      svg: [
        '<svg><g class="nodes">',
        '<g class="node default" id="smarticky-mermaid-1-flowchart-A-0">',
        '<rect class="basic label-container"></rect><g class="label"><rect></rect></g>',
        "</g>",
        '<g class="node default" id="smarticky-mermaid-1-flowchart-B-1">',
        '<rect class="basic label-container"></rect><g class="label"><rect></rect></g>',
        "</g>",
        "</g></svg>",
      ].join(""),
    });

    const result = await renderMermaidDiagram({
      type: "mermaid",
      source: "flowchart TD\nA[灵感] --> B[整理]",
      theme: "light",
    });

    expect(result.html).toContain("灵感");
    expect(result.html).toContain("整理");
    expect(result.html).toContain("smarticky-mermaid-node-label");
  });
});
