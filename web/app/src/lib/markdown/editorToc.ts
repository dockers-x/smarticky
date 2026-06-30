import { $prose } from "@milkdown/kit/utils";
import { Plugin, PluginKey, type EditorState } from "@milkdown/kit/prose/state";
import { Decoration, DecorationSet, type EditorView } from "@milkdown/kit/prose/view";
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

interface EditorTocSnapshot {
  blocks: EditorTocBlock[];
  entries: EditorTocEntry[];
  markerBlocks: EditorTocBlock[];
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

function createEditorTocSnapshot(state: EditorState): EditorTocSnapshot {
  const blocks = topLevelBlocks(state.doc);
  const markerBlocks = blocks.filter(
    (block) => block.type === "paragraph" && isTocMarkerText(block.text),
  );

  return {
    blocks,
    entries: collectEditorTocEntries(blocks),
    markerBlocks,
  };
}

function createEditorTocSignature(snapshot: EditorTocSnapshot): string {
  if (snapshot.markerBlocks.length === 0) return "hidden";

  const markers = snapshot.markerBlocks
    .map((marker) => marker.pos)
    .join(",");
  const entries = snapshot.entries
    .map((entry) => `${entry.pos}:${entry.depth}:${entry.id}:${entry.text}`)
    .join("|");
  return `${markers}::${entries}`;
}

function cleanupTocPreview(node: Element | null): void {
  if (!(node instanceof HTMLElement)) return;
  tocPreviewCleanup.get(node)?.();
  tocPreviewCleanup.delete(node);
}

export function createEditorTocPreview(entries: EditorTocEntry[]): HTMLElement {
  const nav = document.createElement("nav");
  nav.className = "markdown-toc editor-toc-panel";
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
    item.dataset.tocDepth = String(entry.depth);

    const button = document.createElement("button");
    button.type = "button";
    button.className = "editor-toc-button";
    button.dataset.editorHeadingTarget = entry.id;
    button.dataset.tocDepth = String(entry.depth);
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

export function createEditorTocLayer(): HTMLDivElement {
  const layer = document.createElement("div");
  layer.className = "editor-toc-layer";
  layer.contentEditable = "false";
  layer.hidden = true;
  return layer;
}

export function renderEditorTocLayer(
  layer: HTMLElement,
  entries: EditorTocEntry[],
  visible: boolean,
): void {
  cleanupTocPreview(layer.firstElementChild);
  layer.replaceChildren();
  layer.hidden = !visible;

  if (visible) {
    layer.append(createEditorTocPreview(entries));
  }
}

function findEditorTocHost(view: EditorView): HTMLElement | null {
  const host = view.dom.closest(".markdown-editor-host");
  if (host instanceof HTMLElement) return host;
  return view.dom.parentElement;
}

function createEditorTocPluginView(view: EditorView) {
  const host = findEditorTocHost(view);
  if (!host) return {};

  const layer = createEditorTocLayer();
  let signature = "";
  host.prepend(layer);

  const sync = (state: EditorState): void => {
    const snapshot = createEditorTocSnapshot(state);
    const nextSignature = createEditorTocSignature(snapshot);
    if (nextSignature === signature) return;

    signature = nextSignature;
    renderEditorTocLayer(layer, snapshot.entries, snapshot.markerBlocks.length > 0);
  };

  sync(view.state);

  return {
    update(nextView: EditorView) {
      sync(nextView.state);
    },
    destroy() {
      cleanupTocPreview(layer.firstElementChild);
      layer.remove();
    },
  };
}

function createEditorTocDecorations(state: EditorState): DecorationSet {
  const { blocks, entries, markerBlocks } = createEditorTocSnapshot(state);
  if (markerBlocks.length === 0) return DecorationSet.empty;

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

  for (const marker of markerBlocks) {
    if (!marker.node) continue;
    decorations.push(
      Decoration.node(marker.pos, marker.pos + marker.node.nodeSize, {
        class: "editor-toc-source-marker",
        "data-editor-toc-marker": "true",
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
        view: createEditorTocPluginView,
        props: {
          decorations(state) {
            return createEditorTocDecorations(state);
          },
        },
      }),
  );
}
