export type DiagramType = "mermaid" | "drawio";
export type DiagramTheme = "light" | "dark";

export interface DiagramRenderRequest {
  type: DiagramType;
  source: string;
  theme: DiagramTheme;
}

export interface DiagramRenderResult {
  html: string;
}

export interface DiagramRenderer {
  type: DiagramType;
  render(request: DiagramRenderRequest): Promise<DiagramRenderResult>;
}

export interface DiagramRuntimeState {
  pending: number;
  total: number;
  settled: boolean;
}
