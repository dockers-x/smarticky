import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderDrawioDiagram } from "./drawio";

const drawioMock = vi.hoisted(() => ({
  convert: vi.fn(),
}));

vi.mock("./drawio2svg-wrapper.js", () => drawioMock);

describe("renderDrawioDiagram", () => {
  beforeEach(() => {
    drawioMock.convert.mockReset().mockReturnValue("<svg><text>drawio</text></svg>");
  });

  it("converts drawio XML to sanitized SVG HTML", async () => {
    const result = await renderDrawioDiagram({
      type: "drawio",
      source: "<mxfile><diagram>safe</diagram></mxfile>",
      theme: "light",
    });

    expect(drawioMock.convert).toHaveBeenCalledWith(
      "<mxfile><diagram>safe</diagram></mxfile>",
      expect.objectContaining({
        padding: 8,
      }),
    );
    expect(result.html).toContain("diagram-render diagram-render--drawio");
    expect(result.html).toContain("<svg");
    expect(result.html).toContain("drawio");
  });

  it("removes active content from generated SVG", async () => {
    drawioMock.convert.mockReturnValue("<svg><script>alert(1)</script><text>safe</text></svg>");

    const result = await renderDrawioDiagram({
      type: "drawio",
      source: "<mxfile></mxfile>",
      theme: "light",
    });

    expect(result.html).not.toContain("<script");
    expect(result.html).toContain("safe");
  });

  it("passes converter errors through for the runtime error block", async () => {
    drawioMock.convert.mockImplementation(() => {
      throw new Error("bad mxfile");
    });

    await expect(
      renderDrawioDiagram({
        type: "drawio",
        source: "<mxfile>",
        theme: "light",
      }),
    ).rejects.toThrow("bad mxfile");
  });
});
