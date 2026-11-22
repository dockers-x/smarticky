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

export function captureRichTextEscape(pane: HTMLElement, onEscape: (event: KeyboardEvent) => void) {
  function keydown(event: KeyboardEvent): void {
    if (event.key !== "Escape" || event.defaultPrevented || event.isComposing || event.keyCode === 229) return;
    if (event.target instanceof HTMLElement && event.target.classList.contains("ProseMirror")) {
      onEscape(event);
    }
  }

  // ProseMirror prevents the default for Escape before the event reaches window.
  pane.addEventListener("keydown", keydown, true);
  return { destroy: () => pane.removeEventListener("keydown", keydown, true) };
}
