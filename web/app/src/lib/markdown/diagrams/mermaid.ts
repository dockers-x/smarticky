import DOMPurify from "dompurify";
import type { DiagramRenderRequest, DiagramRenderResult, DiagramTheme } from "./types";

type MermaidAPI = typeof import("mermaid").default;

let renderSequence = 0;

async function loadMermaid(): Promise<MermaidAPI> {
  const module = await import("mermaid");
  return module.default;
}

function configureMermaid(mermaid: MermaidAPI, theme: DiagramTheme): void {
  mermaid.initialize({
    startOnLoad: false,
    securityLevel: "strict",
    theme: theme === "dark" ? "dark" : "default",
    themeVariables: {
      background: "transparent",
    },
    flowchart: {
      htmlLabels: false,
      curve: "basis",
    },
  });
}

function sanitizeSvg(svg: string): string {
  return DOMPurify.sanitize(svg, {
    USE_PROFILES: {
      svg: true,
      svgFilters: true,
    },
  });
}

export async function renderMermaidDiagram(
  request: DiagramRenderRequest,
): Promise<DiagramRenderResult> {
  const mermaid = await loadMermaid();
  configureMermaid(mermaid, request.theme);
  await mermaid.parse(request.source);

  renderSequence += 1;
  const id = `smarticky-mermaid-${Date.now()}-${renderSequence}`;
  const { svg } = await mermaid.render(id, request.source);
  const safeSvg = sanitizeSvg(svg);

  return {
    html: `<div class="diagram-render diagram-render--mermaid">${safeSvg}</div>`,
  };
}
