import type { DiagramRenderRequest, DiagramRenderResult } from "./types";

export async function renderDrawioDiagram(
  _request: DiagramRenderRequest,
): Promise<DiagramRenderResult> {
  throw new Error("drawio renderer is not available yet");
}
