export interface MarkdownEditorHandle {
  insertMarkdown(markdown: string, inline?: boolean): void;
  focus(): void;
}
