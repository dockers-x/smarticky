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
    securityLevel: "loose",
    theme: "base",
    themeVariables: mermaidThemeVariables(theme),
    flowchart: {
      htmlLabels: true,
      curve: "basis",
    },
  });
}

function sanitizeSvg(svg: string, source: string): string {
  const normalizedSvg = restoreEmptyFlowchartLabels(
    normalizeMermaidLabels(svg),
    source,
  );

  return DOMPurify.sanitize(normalizedSvg, {
    USE_PROFILES: {
      svg: true,
      svgFilters: true,
    },
  });
}

function normalizeMermaidLabels(svg: string): string {
  if (!/<foreignobject/i.test(svg)) return svg;

  const template = globalThis.document?.createElement("template");
  if (!template) return svg;

  template.innerHTML = svg;
  const root = template.content.querySelector("svg");
  if (!root) return svg;

  root.querySelectorAll("foreignObject, foreignobject").forEach((foreignObject) => {
    foreignObject
      .querySelectorAll("script, style, iframe, object, embed")
      .forEach((node) => node.remove());

    const label = foreignObject.textContent?.replace(/\s+/g, " ").trim();
    if (!label) return;

    const width = Number.parseFloat(foreignObject.getAttribute("width") || "0");
    const height = Number.parseFloat(foreignObject.getAttribute("height") || "0");
    const x = Number.parseFloat(foreignObject.getAttribute("x") || "0");
    const y = Number.parseFloat(foreignObject.getAttribute("y") || "0");
    const text = globalThis.document.createElementNS(
      "http://www.w3.org/2000/svg",
      "text",
    );

    text.setAttribute("x", String(x + (Number.isFinite(width) ? width / 2 : 0)));
    text.setAttribute("y", String(y + (Number.isFinite(height) ? height / 2 : 0)));
    text.setAttribute("text-anchor", "middle");
    text.setAttribute("dominant-baseline", "middle");
    text.textContent = label;
    foreignObject.replaceWith(text);
  });

  return root.outerHTML;
}

function restoreEmptyFlowchartLabels(svg: string, source: string): string {
  if (!/^\s*(flowchart|graph)\b/i.test(source)) return svg;

  const template = globalThis.document?.createElement("template");
  if (!template) return svg;

  template.innerHTML = svg;
  const root = template.content.querySelector("svg");
  if (!root) return svg;

  const labels = extractFlowchartLabels(source);
  let changed = false;

  root.querySelectorAll<SVGGElement>("g.node[id*='-flowchart-']").forEach((node) => {
    if (node.textContent?.replace(/\s+/g, "")) return;

    const nodeID = parseFlowchartNodeID(node.id);
    if (!nodeID) return;

    const label = labels.get(nodeID) ?? nodeID;
    const text = globalThis.document.createElementNS(
      "http://www.w3.org/2000/svg",
      "text",
    );
    text.setAttribute("class", "smarticky-mermaid-node-label");
    text.setAttribute("x", "0");
    text.setAttribute("y", "0");
    text.setAttribute("text-anchor", "middle");
    text.setAttribute("dominant-baseline", "middle");
    text.textContent = label;
    node.append(text);
    changed = true;
  });

  return changed ? root.outerHTML : svg;
}

function parseFlowchartNodeID(value: string): string | null {
  return value.match(/-flowchart-(.+)-\d+$/)?.[1] ?? null;
}

function extractFlowchartLabels(source: string): Map<string, string> {
  const labels = new Map<string, string>();
  const nodePattern =
    /\b([A-Za-z][\w-]*)\s*(?:\[\[([^\]\n]+)\]\]|\[([^\]\n]+)\]|\(\(([^)\n]+)\)\)|\(([^)\n]+)\)|\{\{([^}\n]+)\}\}|\{([^}\n]+)\}|>\s*([^\]\n]+)\])/g;
  let match: RegExpExecArray | null;

  while ((match = nodePattern.exec(source))) {
    const [, id, ...rawLabels] = match;
    const label = rawLabels.find((value) => value !== undefined)?.trim();
    if (!label) continue;
    labels.set(id, label.replace(/^["']|["']$/g, ""));
  }

  return labels;
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
  const safeSvg = sanitizeSvg(svg, source);

  return {
    html: `<div class="diagram-render diagram-render--mermaid">${safeSvg}</div>`,
  };
}
