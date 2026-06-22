import DOMPurify from "dompurify";
import type { DiagramRenderRequest, DiagramRenderResult } from "./types";

type DrawioModule = typeof import("./drawio2svg-wrapper.js");

async function loadDrawio(): Promise<DrawioModule> {
  return import("./drawio2svg-wrapper.js");
}

function sanitizeSvg(svg: string): string {
  return DOMPurify.sanitize(svg, {
    USE_PROFILES: {
      svg: true,
      svgFilters: true,
    },
  });
}

export async function renderDrawioDiagram(
  request: DiagramRenderRequest,
): Promise<DiagramRenderResult> {
  const { convert } = await loadDrawio();
  const svg = convert(request.source, {
    padding: 8,
    fontFamily: "-apple-system, BlinkMacSystemFont, Segoe UI, sans-serif",
  });

  return {
    html: `<div class="diagram-render diagram-render--drawio">${sanitizeSvg(svg)}</div>`,
  };
}
