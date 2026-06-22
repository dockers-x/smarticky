import { renderDrawioDiagram } from "./drawio";
import { renderMermaidDiagram } from "./mermaid";
import type {
  DiagramRenderRequest,
  DiagramRenderResult,
  DiagramRenderer,
  DiagramType,
} from "./types";

const rendererOverrides = new Map<DiagramType, DiagramRenderer>();

const defaultRenderers: Record<DiagramType, DiagramRenderer> = {
  mermaid: {
    type: "mermaid",
    render: renderMermaidDiagram,
  },
  drawio: {
    type: "drawio",
    render: renderDrawioDiagram,
  },
};

export function setDiagramRendererForTest(
  type: DiagramType,
  renderer: DiagramRenderer | null,
): void {
  if (renderer) {
    rendererOverrides.set(type, renderer);
    return;
  }
  rendererOverrides.delete(type);
}

export async function renderDiagram(
  request: DiagramRenderRequest,
): Promise<DiagramRenderResult> {
  const renderer = rendererOverrides.get(request.type) ?? defaultRenderers[request.type];
  return renderer.render(request);
}
