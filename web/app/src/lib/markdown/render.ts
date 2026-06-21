import DOMPurify from "dompurify";
import { Marked } from "marked";
import markedKatex from "marked-katex-extension";

const markedOptions = {
  async: false,
  breaks: false,
  gfm: true,
} as const;

const markdownRenderer = new Marked(
  markedOptions,
  markedKatex({
    throwOnError: false,
  }),
);

export function renderMarkdown(markdown: string): string {
  return DOMPurify.sanitize(markdownRenderer.parse(markdown, markedOptions));
}

export function stripMarkdown(markdown: string): string {
  const container = document.createElement("div");
  container.innerHTML = renderMarkdown(markdown);
  return container.textContent?.trim() ?? "";
}
