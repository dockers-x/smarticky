import type { DiagramType } from "./types";

export function normalizeDiagramType(language: string | null | undefined): DiagramType | null {
  const normalized = (language || "").trim().toLowerCase();
  if (normalized === "mermaid") return "mermaid";
  if (normalized === "drawio" || normalized === "draw.io") return "drawio";
  return null;
}

export function stripDiagramFences(markdown: string): string {
  const lines = markdown.split(/\r?\n/);
  const visibleLines: string[] = [];

  for (let index = 0; index < lines.length; index += 1) {
    const openingFence = lines[index].match(/^```([^\n`]*)\s*$/);
    if (!openingFence || !normalizeDiagramType(openingFence[1])) {
      visibleLines.push(lines[index]);
      continue;
    }

    while (index + 1 < lines.length) {
      index += 1;
      if (/^```\s*$/.test(lines[index])) break;
    }
  }

  return visibleLines.join("\n").replace(/\n{3,}/g, "\n\n").trim();
}
