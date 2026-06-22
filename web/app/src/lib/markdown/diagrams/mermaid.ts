import DOMPurify from "dompurify";
import type { DiagramRenderRequest, DiagramRenderResult, DiagramTheme } from "./types";
import { materializeMermaidSource } from "./mermaidSyntax";

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

function isMissingDiagramTypeError(error: unknown): boolean {
  const message = error instanceof Error ? error.message : String(error);
  return message.toLowerCase().includes("no diagram type detected");
}

async function parseMermaidSource(
  mermaid: MermaidAPI,
  source: string,
): Promise<string> {
  try {
    await mermaid.parse(source);
    return source;
  } catch (error: unknown) {
    if (!isMissingDiagramTypeError(error)) throw error;
  }

  const flowchartSource = materializeMermaidSource("flowchart", source);
  await mermaid.parse(flowchartSource);
  return flowchartSource;
}

export async function renderMermaidDiagram(
  request: DiagramRenderRequest,
): Promise<DiagramRenderResult> {
  const mermaid = await loadMermaid();
  configureMermaid(mermaid, request.theme);
  const source = await parseMermaidSource(mermaid, request.source);

  renderSequence += 1;
  const id = `smarticky-mermaid-${Date.now()}-${renderSequence}`;
  const { svg } = await mermaid.render(id, source);
  const safeSvg = sanitizeSvg(svg);

  return {
    html: `<div class="diagram-render diagram-render--mermaid">${safeSvg}</div>`,
  };
}
