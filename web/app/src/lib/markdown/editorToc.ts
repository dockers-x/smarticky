import { $prose } from "@milkdown/kit/utils";
import { Plugin, PluginKey, type EditorState } from "@milkdown/kit/prose/state";
import { Decoration, DecorationSet } from "@milkdown/kit/prose/view";
import type { Node as ProseNode } from "@milkdown/kit/prose/model";
import {
  escapeHTML,
  isTocMarkerText,
  slugifyHeading,
  type TocEntry,
} from "./toc";

export interface EditorTocBlock {
  pos: number;
  type: string;
  text: string;
  level?: number;
  node?: ProseNode;
}

export interface EditorTocEntry extends TocEntry {
  pos: number;
}

const editorTocPluginKey = new PluginKey("smarticky-editor-toc");
const tocPreviewCleanup = new WeakMap<HTMLElement, () => void>();

function normalizeBlockText(value: string): string {
  return value.replace(/\u200B/g, "").replace(/\s+/g, " ").trim();
}

function uniqueID(base: string, seen: Map<string, number>): string {
  const current = seen.get(base) ?? 0;
  seen.set(base, current + 1);
  return current === 0 ? base : `${base}-${current + 1}`;
}

function selectorValue(value: string): string {
  return value.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
}

function headingLevel(node: ProseNode): number {
  const level = Number(node.attrs.level);
  return Number.isFinite(level) && level >= 1 && level <= 6 ? level : 1;
}

function topLevelBlocks(doc: ProseNode): EditorTocBlock[] {
  const blocks: EditorTocBlock[] = [];
  doc.forEach((node, offset) => {
    blocks.push({
      level: node.type.name === "heading" ? headingLevel(node) : undefined,
      node,
      pos: offset,
      text: normalizeBlockText(node.textContent),
      type: node.type.name,
    });
  });
  return blocks;
}

export function collectEditorTocEntries(blocks: EditorTocBlock[]): EditorTocEntry[] {
  const seen = new Map<string, number>();
  const entries: EditorTocEntry[] = [];

  for (const block of blocks) {
    if (block.type !== "heading") continue;
    const text = block.text.replace(/\s+/g, " ").trim();
    if (!text) continue;
    const id = uniqueID(slugifyHeading(text), seen);
    entries.push({
      depth: block.level ?? 1,
      id,
      pos: block.pos,
      text,
    });
  }

  return entries;
}

export function createEditorTocPreview(entries: EditorTocEntry[]): HTMLElement {
  const nav = document.createElement("nav");
  nav.className = "markdown-toc editor-toc-float";
  nav.contentEditable = "false";
  nav.setAttribute("aria-label", "Table of contents");

  if (entries.length === 0) {
    const empty = document.createElement("p");
    empty.className = "markdown-toc__empty";
    empty.textContent = "No headings yet";
    nav.append(empty);
    return nav;
  }

  const list = document.createElement("ol");
  list.className = "markdown-toc__list";
  const listeners: Array<() => void> = [];

  for (const entry of entries) {
    const item = document.createElement("li");
    item.className = `markdown-toc__item markdown-toc__item--depth-${entry.depth}`;

    const button = document.createElement("button");
    button.type = "button";
    button.className = "editor-toc-button";
    button.dataset.editorHeadingTarget = entry.id;
    button.innerHTML = escapeHTML(entry.text);
    const handleClick = (event: MouseEvent): void => {
      event.preventDefault();
      event.stopPropagation();
      const root = nav.closest(".markdown-editor-host") ?? nav.parentElement;
      const target = root?.querySelector<HTMLElement>(
        `[data-editor-heading-id="${selectorValue(entry.id)}"]`,
      );
      target?.scrollIntoView({ behavior: "smooth", block: "start" });
    };
    button.addEventListener("click", handleClick);
    listeners.push(() => button.removeEventListener("click", handleClick));

    item.append(button);
    list.append(item);
  }

  nav.append(list);
  tocPreviewCleanup.set(nav, () => {
    for (const cleanup of listeners) cleanup();
  });
  return nav;
}

function createEditorTocDecorations(state: EditorState): DecorationSet {
  const blocks = topLevelBlocks(state.doc);
  const markerBlocks = blocks.filter(
    (block) => block.type === "paragraph" && isTocMarkerText(block.text),
  );
  if (markerBlocks.length === 0) return DecorationSet.empty;

  const entries = collectEditorTocEntries(blocks);
  const decorations: Decoration[] = [];

  for (const entry of entries) {
    const block = blocks.find((candidate) => candidate.pos === entry.pos);
    if (!block?.node) continue;
    decorations.push(
      Decoration.node(block.pos, block.pos + block.node.nodeSize, {
        "data-editor-heading-id": entry.id,
      }),
    );
  }

  const [firstMarker] = markerBlocks;
  if (firstMarker) {
    decorations.push(
      Decoration.widget(firstMarker.pos, () => createEditorTocPreview(entries), {
        key: `smarticky-editor-toc-${entries.map((entry) => entry.id).join("-")}`,
        side: -1,
        ignoreSelection: true,
        destroy: (node: Node) => {
          if (node instanceof HTMLElement) {
            tocPreviewCleanup.get(node)?.();
            tocPreviewCleanup.delete(node);
          }
        },
        stopEvent: (event: Event) => {
          const target = event.target;
          return (
            target instanceof Element &&
            Boolean(target.closest(".editor-toc-button"))
          );
        },
      }),
    );
  }

  for (const marker of markerBlocks) {
    if (!marker.node) continue;
    decorations.push(
      Decoration.node(marker.pos, marker.pos + marker.node.nodeSize, {
        class: "editor-toc-source-hidden",
        "aria-hidden": "true",
      }),
    );
  }

  return DecorationSet.create(state.doc, decorations);
}

export function createEditorTocPlugin() {
  return $prose(
    () =>
      new Plugin({
        key: editorTocPluginKey,
        props: {
          decorations(state) {
            return createEditorTocDecorations(state);
          },
        },
      }),
  );
}
