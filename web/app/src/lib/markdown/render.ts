import DOMPurify from "dompurify";
import { Marked, Renderer, type Tokens } from "marked";
import markedKatex from "marked-katex-extension";
import {
  normalizeDiagramType,
  prepareDiagramSource,
  stripDiagramFences,
} from "./diagrams/fences";
import { createDiagramPlaceholder } from "./diagrams/placeholders";
import {
  createWhiteboardPlaceholder,
  extractWhiteboardID,
  isWhiteboardFenceLanguage,
} from "./whiteboards";
import { transformCodeGroups } from "./codeGroups";
import { isProtectedAttachmentURL, protectedImagePlaceholderSrc } from "./protectedImages";

const markedOptions = {
  async: false,
  breaks: false,
  gfm: true,
} as const;

const fallbackRenderer = new Renderer();

function escapeAttribute(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

const markdownRenderer = new Marked(
  markedOptions,
  markedKatex({
    throwOnError: false,
  }),
  {
    renderer: {
      code(token: Tokens.Code): string {
        if (isWhiteboardFenceLanguage(token.lang)) {
          const whiteboardID = extractWhiteboardID(token.text);
          return whiteboardID
            ? createWhiteboardPlaceholder(whiteboardID)
            : fallbackRenderer.code(token);
        }

        const diagramType = normalizeDiagramType(token.lang);
        if (diagramType && token.text.trim()) {
          return createDiagramPlaceholder(
            diagramType,
            prepareDiagramSource(token.lang, token.text),
          );
        }
        return fallbackRenderer.code(token);
      },
      image(token: Tokens.Image): string {
        const href = token.href.trim();
        const alt = token.text.trim();
        if (!href) {
          return `<span class="markdown-image-placeholder">${escapeAttribute(alt || "Image URL missing")}</span>`;
        }

        if (isProtectedAttachmentURL(href)) {
          const title = token.title
            ? ` title="${escapeAttribute(token.title)}"`
            : "";
          return `<img src="${protectedImagePlaceholderSrc}" data-auth-image="true" data-auth-src="${escapeAttribute(href)}" alt="${escapeAttribute(alt)}"${title}>`;
        }

        return fallbackRenderer.image(token);
      },
    },
  },
);

export function renderMarkdown(markdown: string): string {
  return DOMPurify.sanitize(
    markdownRenderer.parse(transformCodeGroups(markdown), markedOptions),
  );
}

export function stripMarkdown(markdown: string): string {
  const container = document.createElement("div");
  container.innerHTML = renderMarkdown(stripDiagramFences(markdown));
  return container.textContent?.trim() ?? "";
}
