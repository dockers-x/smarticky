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
import {
  collectTocEntries,
  renderTableOfContents,
  replaceTocMarkers,
} from "./toc";

const markedOptions = {
  async: false,
  breaks: false,
  gfm: true,
} as const;

const fallbackRenderer = new Renderer();
let activeHeadingIDs: string[] | null = null;

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
      heading(token: Tokens.Heading): string {
        const id = activeHeadingIDs?.shift();
        const content = this.parser.parseInline(token.tokens);
        const idAttribute = id ? ` id="${escapeAttribute(id)}"` : "";
        return `<h${token.depth}${idAttribute}>${content}</h${token.depth}>\n`;
      },
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

export interface RenderMarkdownOptions {
  toc?: "include" | "omit";
}

export function renderMarkdown(
  markdown: string,
  options: RenderMarkdownOptions = {},
): string {
  const transformed = transformCodeGroups(markdown);
  const tokens = markdownRenderer.lexer(transformed, markedOptions);
  const tocEntries = collectTocEntries(tokens);
  const { tokens: tokensWithToc } = replaceTocMarkers(
    tokens,
    options.toc === "omit" ? "" : renderTableOfContents(tocEntries),
  );
  activeHeadingIDs = tocEntries.map((entry) => entry.id);
  try {
    return DOMPurify.sanitize(markdownRenderer.parser(tokensWithToc));
  } finally {
    activeHeadingIDs = null;
  }
}

export function stripMarkdown(markdown: string): string {
  const container = document.createElement("div");
  container.innerHTML = renderMarkdown(stripDiagramFences(markdown), { toc: "omit" });
  return container.textContent?.replace(/\s+/g, " ").trim() ?? "";
}
