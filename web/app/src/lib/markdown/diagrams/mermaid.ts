import type { DiagramRenderRequest, DiagramRenderResult } from "./types";

export async function renderMermaidDiagram(
  _request: DiagramRenderRequest,
): Promise<DiagramRenderResult> {
  throw new Error("Mermaid renderer is not available yet");
}
