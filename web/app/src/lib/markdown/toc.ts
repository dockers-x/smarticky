import type { Token, Tokens } from "marked";

export interface TocEntry {
  depth: number;
  id: string;
  text: string;
}

export function isTocMarkerText(value: string): boolean {
  return /^\s*\[toc\]\s*$/i.test(value);
}

export function escapeHTML(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

export function slugifyHeading(value: string): string {
  const normalized = value
    .normalize("NFKD")
    .toLowerCase()
    .replace(/[^\p{Letter}\p{Number}\s_-]+/gu, "")
    .trim()
    .replace(/[\s_-]+/g, "-")
    .replace(/^-+|-+$/g, "");

  return normalized || "section";
}

function tokenText(token: Token): string {
  if ("tokens" in token && token.tokens?.length) {
    return token.tokens.map(tokenText).join("");
  }

  switch (token.type) {
    case "br":
      return " ";
    case "html":
      return "";
    case "image":
      return token.text;
    case "text":
    case "escape":
    case "codespan":
      return token.text;
    default:
      return "text" in token && typeof token.text === "string" ? token.text : "";
  }
}

function uniqueID(base: string, seen: Map<string, number>): string {
  const current = seen.get(base) ?? 0;
  seen.set(base, current + 1);
  return current === 0 ? base : `${base}-${current + 1}`;
}

export function collectTocEntries(tokens: Token[]): TocEntry[] {
  const seen = new Map<string, number>();
  const entries: TocEntry[] = [];

  for (const token of tokens) {
    if (token.type !== "heading") continue;

    const text = tokenText(token).replace(/\s+/g, " ").trim();
    if (!text) continue;

    const id = uniqueID(slugifyHeading(text), seen);
    entries.push({
      depth: token.depth,
      id,
      text,
    });
  }

  return entries;
}

export function renderTableOfContents(entries: TocEntry[]): string {
  const items = entries
    .map(
      (entry) =>
        `<li class="markdown-toc__item markdown-toc__item--depth-${entry.depth}"><a href="#${escapeHTML(entry.id)}">${escapeHTML(entry.text)}</a></li>`,
    )
    .join("");

  return [
    '<nav class="markdown-toc" aria-label="Table of contents">',
    items
      ? `<ol class="markdown-toc__list">${items}</ol>`
      : '<p class="markdown-toc__empty">No headings yet</p>',
    "</nav>",
  ].join("");
}

export function replaceTocMarkers(
  tokens: Token[],
  tocHTML: string,
): { tokens: Token[]; found: boolean } {
  let found = false;
  const nextTokens = tokens.map((token): Token => {
    if (
      token.type === "paragraph" &&
      isTocMarkerText(token.text) &&
      (token.tokens?.length ?? 0) === 1
    ) {
      found = true;
      return {
        type: "html",
        raw: token.raw,
        text: tocHTML,
        block: true,
        pre: false,
      } satisfies Tokens.HTML;
    }
    return token;
  });

  return { tokens: nextTokens, found };
}
