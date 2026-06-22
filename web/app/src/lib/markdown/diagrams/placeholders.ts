import type { DiagramType } from "./types";

export function encodeDiagramSource(source: string): string {
  return btoa(unescape(encodeURIComponent(source)));
}

export function decodeDiagramSource(encoded: string): string {
  return decodeURIComponent(escape(atob(encoded)));
}

export function createDiagramPlaceholder(type: DiagramType, source: string): string {
  const encodedSource = encodeDiagramSource(source);
  const label = type === "mermaid" ? "Mermaid" : "drawio";

  return [
    `<div class="diagram-block diagram-block--${type}" data-diagram-placeholder="true" data-diagram-type="${type}" data-diagram-source="${encodedSource}">`,
    `<div class="diagram-loading" aria-live="polite">Rendering ${label} diagram...</div>`,
    "</div>",
  ].join("");
}
