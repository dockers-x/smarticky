import { $prose } from "@milkdown/kit/utils";
import { Plugin, PluginKey, type EditorState } from "@milkdown/kit/prose/state";
import { Decoration, DecorationSet } from "@milkdown/kit/prose/view";
import type { Node as ProseNode } from "@milkdown/kit/prose/model";
import {
  attachCodeGroupTabs,
  extractCodeGroupSources,
  renderCodeGroup,
  type CodeGroupBlock,
  type CodeGroupSourceBlock,
  type CodeGroupSourceEditRequest,
} from "./codeGroups";

interface TopLevelBlock {
  node: ProseNode;
  pos: number;
  text: string;
}

const codeGroupPluginKey = new PluginKey("smarticky-code-groups");
const previewCleanup = new WeakMap<HTMLElement, () => void>();

export type ReplaceCodeGroupSource = (
  request: CodeGroupSourceEditRequest,
  nextRaw: string,
) => string | null;

function hashString(value: string): string {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) >>> 0;
  }
  return hash.toString(36);
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function normalizeBlockText(value: string): string {
  return value.replace(/\u200B/g, "").replace(/\s+/g, " ").trim();
}

function isCodeGroupOpener(text: string, group: CodeGroupBlock): boolean {
  const pattern = new RegExp(
    `^\\s*${escapeRegExp(group.marker)}\\s+${group.kind}(?:[#\\s].*)?$`,
  );
  return pattern.test(text);
}

function isCodeGroupCloser(text: string, group: CodeGroupBlock): boolean {
  return text === group.marker;
}

function topLevelBlocks(doc: ProseNode): TopLevelBlock[] {
  const blocks: TopLevelBlock[] = [];
  doc.forEach((node, offset) => {
    blocks.push({
      node,
      pos: offset,
      text: normalizeBlockText(node.textContent),
    });
  });
  return blocks;
}

function codeGroupSourceID(group: CodeGroupSourceBlock): string {
  return `smarticky-code-group-source-${group.startLine}-${group.endLine}-${hashString(group.raw)}-${hashString(JSON.stringify(group.items))}`;
}

function createCodeGroupPreview(
  group: CodeGroupSourceBlock,
  sourceID: string,
  replaceSource: ReplaceCodeGroupSource,
): HTMLElement {
  const preview = document.createElement("div");
  preview.className = "editor-code-group-preview";
  preview.contentEditable = "false";
  preview.dataset.codeGroupKind = group.kind;
  preview.dataset.codeGroupStartLine = String(group.startLine);
  preview.dataset.codeGroupSource = sourceID;

  const sourceModeButton = document.createElement("button");
  sourceModeButton.type = "button";
  sourceModeButton.className = "editor-code-group-source-toggle";
  sourceModeButton.textContent = "Source";
  sourceModeButton.title = "Edit this code group source";

  const content = document.createElement("div");
  content.className = "editor-code-group-preview__content";
  content.innerHTML = renderCodeGroup(group.items);

  const sourceEditor = document.createElement("div");
  sourceEditor.className = "editor-code-group-source-editor";
  sourceEditor.hidden = true;

  const textarea = document.createElement("textarea");
  const errorID = `${sourceID}-error`;
  textarea.className = "editor-code-group-source-textarea";
  textarea.spellcheck = false;
  textarea.value = group.raw;
  textarea.setAttribute("aria-label", "Code group source");
  textarea.setAttribute("aria-describedby", errorID);

  const error = document.createElement("div");
  error.id = errorID;
  error.className = "editor-code-group-source-error";
  error.hidden = true;
  error.setAttribute("role", "alert");
  error.setAttribute("aria-live", "polite");

  const actions = document.createElement("div");
  actions.className = "editor-code-group-source-actions";

  const saveButton = document.createElement("button");
  saveButton.type = "button";
  saveButton.className = "editor-code-group-source-save";
  saveButton.textContent = "Save";

  const cancelButton = document.createElement("button");
  cancelButton.type = "button";
  cancelButton.className = "editor-code-group-source-cancel";
  cancelButton.textContent = "Cancel";

  actions.append(saveButton, cancelButton);
  sourceEditor.append(textarea, actions, error);
  preview.append(sourceModeButton, content, sourceEditor);

  const detach = attachCodeGroupTabs(preview);
  let focusTimer: ReturnType<typeof window.setTimeout> | null = null;

  const showPreview = (): void => {
    textarea.value = group.raw;
    error.hidden = true;
    error.textContent = "";
    sourceEditor.hidden = true;
    content.hidden = false;
    sourceModeButton.hidden = false;
  };

  const showSourceEditor = (): void => {
    textarea.value = group.raw;
    error.hidden = true;
    error.textContent = "";
    sourceEditor.hidden = false;
    content.hidden = true;
    sourceModeButton.hidden = true;
    if (focusTimer) window.clearTimeout(focusTimer);
    focusTimer = window.setTimeout(() => {
      focusTimer = null;
      if (!textarea.isConnected) return;
      textarea.focus();
      textarea.setSelectionRange(0, textarea.value.length);
    }, 0);
  };

  const handleSourceModeRequest = (event: MouseEvent): void => {
    event.preventDefault();
    event.stopPropagation();
    showSourceEditor();
  };
  const handleSave = (event: MouseEvent): void => {
    event.preventDefault();
    event.stopPropagation();
    const message = replaceSource(
      {
        sourceID,
        startLine: group.startLine,
        endLine: group.endLine,
        raw: group.raw,
        signature: group.signature,
      },
      textarea.value,
    );
    if (message) {
      error.textContent = message;
      error.hidden = false;
    }
  };
  const handleCancel = (event: MouseEvent): void => {
    event.preventDefault();
    event.stopPropagation();
    showPreview();
  };
  sourceModeButton.addEventListener("click", handleSourceModeRequest);
  saveButton.addEventListener("click", handleSave);
  cancelButton.addEventListener("click", handleCancel);
  previewCleanup.set(preview, () => {
    if (focusTimer) {
      window.clearTimeout(focusTimer);
      focusTimer = null;
    }
    detach();
    sourceModeButton.removeEventListener("click", handleSourceModeRequest);
    saveButton.removeEventListener("click", handleSave);
    cancelButton.removeEventListener("click", handleCancel);
  });
  return preview;
}

function createCodeGroupDecorations(
  state: EditorState,
  markdown: string,
  replaceSource: ReplaceCodeGroupSource,
): DecorationSet {
  const groups = extractCodeGroupSources(markdown).filter((group) =>
    Boolean(renderCodeGroup(group.items)),
  );
  if (groups.length === 0) return DecorationSet.empty;

  const blocks = topLevelBlocks(state.doc);
  if (blocks.length === 0) return DecorationSet.empty;

  const decorations: Decoration[] = [];
  let cursor = 0;

  for (const group of groups) {
    const openerIndex = blocks.findIndex(
      (block, index) => index >= cursor && isCodeGroupOpener(block.text, group),
    );
    if (openerIndex < 0) continue;

    const closerIndex = blocks.findIndex(
      (block, index) => index > openerIndex && isCodeGroupCloser(block.text, group),
    );
    if (closerIndex < 0) {
      cursor = openerIndex + 1;
      continue;
    }

    const opener = blocks[openerIndex];
    const sourceID = codeGroupSourceID(group);
    decorations.push(
      Decoration.widget(opener.pos, () => createCodeGroupPreview(group, sourceID, replaceSource), {
        key: `smarticky-code-group-${group.startLine}-${group.endLine}-${hashString(group.raw)}-${hashString(JSON.stringify(group.items))}`,
        side: -1,
        ignoreSelection: true,
        destroy: (node: Node) => {
          if (node instanceof HTMLElement) {
            previewCleanup.get(node)?.();
            previewCleanup.delete(node);
          }
        },
        stopEvent: (event: Event) => {
          const target = event.target;
          return (
            target instanceof Element &&
            Boolean(
              target.closest(
                ".markdown-code-tab, .editor-code-group-source-toggle, .editor-code-group-source-editor",
              ),
            )
          );
        },
      }),
    );

    for (let index = openerIndex; index <= closerIndex; index += 1) {
      const block = blocks[index];
      decorations.push(
        Decoration.node(block.pos, block.pos + block.node.nodeSize, {
          class: "editor-code-group-source-hidden",
          "aria-hidden": "true",
          "data-code-group-source": sourceID,
        }),
      );
    }
    cursor = closerIndex + 1;
  }

  if (decorations.length === 0) return DecorationSet.empty;
  return DecorationSet.create(state.doc, decorations);
}

export function createCodeGroupEditorPlugin(
  getMarkdown: () => string,
  replaceSource: ReplaceCodeGroupSource,
) {
  return $prose(
    () =>
      new Plugin({
        key: codeGroupPluginKey,
        props: {
          decorations(state) {
            return createCodeGroupDecorations(state, getMarkdown(), replaceSource);
          },
        },
      }),
  );
}
