export interface MarkdownEditorHandle {
  readonly noteID: string;
  getMarkdown(): string;
  insertMarkdown(markdown: string, inline?: boolean): void;
  focus(): void;
}

export function captureEditorMarkdown(
  editor: MarkdownEditorHandle | null,
  noteID: string,
): string | null {
  return editor?.noteID === noteID ? editor.getMarkdown() : null;
}
