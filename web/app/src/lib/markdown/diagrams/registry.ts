import type {
  DiagramRenderRequest,
  DiagramRenderResult,
  DiagramRenderer,
  DiagramType,
} from "./types";

const rendererOverrides = new Map<DiagramType, DiagramRenderer>();
const defaultRendererCache = new Map<DiagramType, DiagramRenderer>();

const defaultRendererLoaders: Record<DiagramType, () => Promise<DiagramRenderer>> = {
  mermaid: async () => {
    const { renderMermaidDiagram } = await import("./mermaid");
    return {
      type: "mermaid",
      render: renderMermaidDiagram,
    };
  },
  drawio: async () => {
    const { renderDrawioDiagram } = await import("./drawio");
    return {
      type: "drawio",
      render: renderDrawioDiagram,
    };
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

async function loadDefaultRenderer(type: DiagramType): Promise<DiagramRenderer> {
  const cached = defaultRendererCache.get(type);
  if (cached) return cached;

  const renderer = await defaultRendererLoaders[type]();
  defaultRendererCache.set(type, renderer);
  return renderer;
}

export async function renderDiagram(
  request: DiagramRenderRequest,
): Promise<DiagramRenderResult> {
  const renderer =
    rendererOverrides.get(request.type) ?? (await loadDefaultRenderer(request.type));
  return renderer.render(request);
}
