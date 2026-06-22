import DOMPurify from "dompurify";
import { Marked, Renderer, type Tokens } from "marked";
import markedKatex from "marked-katex-extension";
import {
  normalizeDiagramType,
  prepareDiagramSource,
  stripDiagramFences,
} from "./diagrams/fences";
import { createDiagramPlaceholder } from "./diagrams/placeholders";

const markedOptions = {
  async: false,
  breaks: false,
  gfm: true,
} as const;

const fallbackRenderer = new Renderer();

const markdownRenderer = new Marked(
  markedOptions,
  markedKatex({
    throwOnError: false,
  }),
  {
    renderer: {
      code(token: Tokens.Code): string {
        const diagramType = normalizeDiagramType(token.lang);
        if (diagramType && token.text.trim()) {
          return createDiagramPlaceholder(
            diagramType,
            prepareDiagramSource(token.lang, token.text),
          );
        }
        return fallbackRenderer.code(token);
      },
    },
  },
);

export function renderMarkdown(markdown: string): string {
  return DOMPurify.sanitize(markdownRenderer.parse(markdown, markedOptions));
}

export function stripMarkdown(markdown: string): string {
  const container = document.createElement("div");
  container.innerHTML = renderMarkdown(stripDiagramFences(markdown));
  return container.textContent?.trim() ?? "";
}
