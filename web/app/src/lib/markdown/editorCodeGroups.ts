import { $prose } from "@milkdown/kit/utils";
import { Plugin, PluginKey, type EditorState } from "@milkdown/kit/prose/state";
import { Decoration, DecorationSet } from "@milkdown/kit/prose/view";
import type { Node as ProseNode } from "@milkdown/kit/prose/model";
import {
  attachCodeGroupTabs,
  extractCodeGroups,
  renderCodeGroup,
  type CodeGroupBlock,
} from "./codeGroups";

interface TopLevelBlock {
  node: ProseNode;
  pos: number;
  text: string;
}

const codeGroupPluginKey = new PluginKey("smarticky-code-groups");
const previewCleanup = new WeakMap<HTMLElement, () => void>();

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

function codeGroupSourceID(group: CodeGroupBlock): string {
  return `smarticky-code-group-source-${group.startLine}-${group.endLine}-${hashString(JSON.stringify(group.items))}`;
}

function createCodeGroupPreview(
  group: CodeGroupBlock,
  sourceID: string,
  requestSourceMode: () => void,
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
  sourceModeButton.textContent = "Source mode";
  sourceModeButton.title = "Open the full Markdown source editor";

  const content = document.createElement("div");
  content.className = "editor-code-group-preview__content";
  content.innerHTML = renderCodeGroup(group.items);

  preview.append(sourceModeButton, content);
  const detach = attachCodeGroupTabs(preview);
  const handleSourceModeRequest = (event: MouseEvent): void => {
    event.preventDefault();
    event.stopPropagation();
    requestSourceMode();
  };
  sourceModeButton.addEventListener("click", handleSourceModeRequest);
  previewCleanup.set(preview, () => {
    detach();
    sourceModeButton.removeEventListener("click", handleSourceModeRequest);
  });
  return preview;
}

function createCodeGroupDecorations(
  state: EditorState,
  markdown: string,
  requestSourceMode: () => void,
): DecorationSet {
  const groups = extractCodeGroups(markdown);
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
      Decoration.widget(opener.pos, () => createCodeGroupPreview(group, sourceID, requestSourceMode), {
        key: `smarticky-code-group-${group.startLine}-${group.endLine}-${hashString(JSON.stringify(group.items))}`,
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
            Boolean(target.closest(".markdown-code-tab, .editor-code-group-source-toggle"))
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
  requestSourceMode: () => void = () => {},
) {
  return $prose(
    () =>
      new Plugin({
        key: codeGroupPluginKey,
        props: {
          decorations(state) {
            return createCodeGroupDecorations(state, getMarkdown(), requestSourceMode);
          },
        },
      }),
  );
}
