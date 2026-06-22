import DOMPurify from "dompurify";
import type { DiagramRenderRequest, DiagramRenderResult, DiagramTheme } from "./types";
import { materializeMermaidSource } from "./mermaidSyntax";

type MermaidAPI = typeof import("mermaid").default;

let renderSequence = 0;

async function loadMermaid(): Promise<MermaidAPI> {
  const module = await import("mermaid");
  return module.default;
}

function mermaidThemeVariables(theme: DiagramTheme): Record<string, string> {
  if (theme === "dark") {
    return {
      background: "transparent",
      mainBkg: "#26231d",
      secondBkg: "#302b22",
      tertiaryColor: "#1f1e1a",
      primaryColor: "#26231d",
      primaryTextColor: "#f6f1e8",
      primaryBorderColor: "#f97316",
      secondaryColor: "#302b22",
      secondaryTextColor: "#f6f1e8",
      secondaryBorderColor: "#a98b63",
      tertiaryTextColor: "#f6f1e8",
      tertiaryBorderColor: "#71624b",
      textColor: "#f6f1e8",
      nodeTextColor: "#f6f1e8",
      lineColor: "#d8cbb7",
      edgeLabelBackground: "#1f1e1a",
      clusterBkg: "#1f1e1a",
      clusterBorder: "#71624b",
      titleColor: "#f6f1e8",
      actorBkg: "#26231d",
      actorBorder: "#f97316",
      actorTextColor: "#f6f1e8",
      actorLineColor: "#d8cbb7",
      signalColor: "#d8cbb7",
      signalTextColor: "#f6f1e8",
      labelBoxBkgColor: "#26231d",
      labelBoxBorderColor: "#f97316",
      labelTextColor: "#f6f1e8",
      loopTextColor: "#f6f1e8",
      noteBkgColor: "#302b22",
      noteBorderColor: "#a98b63",
      noteTextColor: "#f6f1e8",
      activationBkgColor: "#302b22",
      activationBorderColor: "#d8cbb7",
    };
  }

  return {
    background: "transparent",
    mainBkg: "#fff8f0",
    secondBkg: "#fff2e3",
    tertiaryColor: "#fffdf9",
    primaryColor: "#fff8f0",
    primaryTextColor: "#27231f",
    primaryBorderColor: "#e85d1c",
    secondaryColor: "#fff2e3",
    secondaryTextColor: "#27231f",
    secondaryBorderColor: "#f0a56d",
    tertiaryTextColor: "#27231f",
    tertiaryBorderColor: "#d8c9ba",
    textColor: "#27231f",
    nodeTextColor: "#27231f",
    lineColor: "#675f57",
    edgeLabelBackground: "#fffdf9",
    clusterBkg: "#fffdf9",
    clusterBorder: "#e3d6c8",
    titleColor: "#27231f",
    actorBkg: "#fff8f0",
    actorBorder: "#e85d1c",
    actorTextColor: "#27231f",
    actorLineColor: "#675f57",
    signalColor: "#675f57",
    signalTextColor: "#27231f",
    labelBoxBkgColor: "#fff8f0",
    labelBoxBorderColor: "#e85d1c",
    labelTextColor: "#27231f",
    loopTextColor: "#27231f",
    noteBkgColor: "#fff2e3",
    noteBorderColor: "#f0a56d",
    noteTextColor: "#27231f",
    activationBkgColor: "#fff2e3",
    activationBorderColor: "#675f57",
  };
}

function configureMermaid(mermaid: MermaidAPI, theme: DiagramTheme): void {
  mermaid.initialize({
    startOnLoad: false,
    securityLevel: "strict",
    theme: "base",
    themeVariables: mermaidThemeVariables(theme),
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
