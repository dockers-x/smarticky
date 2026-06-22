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
        securityLevel: "strict",
        theme: "default",
      }),
    );
    expect(mermaidMock.parse).toHaveBeenCalledWith("flowchart TD\nA --> B");
    expect(mermaidMock.render).toHaveBeenCalled();
    expect(result.html).toContain("diagram-render diagram-render--mermaid");
    expect(result.html).toContain("<svg");
    expect(result.html).toContain("diagram");
  });

  it("uses Mermaid dark theme for dark diagram theme", async () => {
    await renderMermaidDiagram({
      type: "mermaid",
      source: "sequenceDiagram\nA->>B: hi",
      theme: "dark",
    });

    expect(mermaidMock.initialize).toHaveBeenCalledWith(
      expect.objectContaining({
        theme: "dark",
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
});
