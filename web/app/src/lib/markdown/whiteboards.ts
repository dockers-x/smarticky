const whiteboardLinePattern = /^(?:whiteboard|id)\s*:\s*([0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12})\s*$/im;
const bareWhiteboardIDPattern = /^([0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12})$/i;

export function isWhiteboardFenceLanguage(
  language: string | null | undefined,
): boolean {
  return (language || "").trim().toLowerCase() === "excalidraw";
}

export function extractWhiteboardID(source: string): string | null {
  const trimmed = source.trim();
  const lineMatch = trimmed.match(whiteboardLinePattern);
  if (lineMatch) return lineMatch[1];

  const bareMatch = trimmed.match(bareWhiteboardIDPattern);
  return bareMatch ? bareMatch[1] : null;
}

export function createWhiteboardReferenceFence(whiteboardID: string): string {
  return ["```excalidraw", `whiteboard: ${whiteboardID}`, "```"].join("\n");
}

export interface WhiteboardReferenceRemoval {
  markdown: string;
  removedCount: number;
}

function fenceLanguage(info: string): string {
  return info.trim().split(/\s+/)[0] || "";
}

function isClosingFence(line: string, marker: string): boolean {
  const match = line.match(/^( {0,3})(`{3,}|~{3,})\s*$/);
  if (!match) return false;
  const closingMarker = match[2];
  return closingMarker[0] === marker[0] && closingMarker.length >= marker.length;
}

function normalizeRemovedReferenceSpacing(markdown: string): string {
  const compacted = markdown.replace(/\n{3,}/g, "\n\n");
  if (!compacted.trim()) return "";
  return compacted.replace(/^\n+/, "").replace(/\n+$/, "");
}

export function removeWhiteboardReferenceFences(
  markdown: string,
  whiteboardID: string,
): WhiteboardReferenceRemoval {
  const normalizedWhiteboardID = whiteboardID.toLowerCase();
  const lines = markdown.split(/\r\n|\n|\r/);
  const keptLines: string[] = [];
  let removedCount = 0;

  for (let index = 0; index < lines.length; index += 1) {
    const openingFence = lines[index].match(/^( {0,3})(`{3,}|~{3,})(.*)$/);
    if (!openingFence || !isWhiteboardFenceLanguage(fenceLanguage(openingFence[3]))) {
      keptLines.push(lines[index]);
      continue;
    }

    const marker = openingFence[2];
    const blockLines = [lines[index]];
    const sourceLines: string[] = [];
    let closed = false;

    while (index + 1 < lines.length) {
      index += 1;
      blockLines.push(lines[index]);
      if (isClosingFence(lines[index], marker)) {
        closed = true;
        break;
      }
      sourceLines.push(lines[index]);
    }

    const referencedID = extractWhiteboardID(sourceLines.join("\n"));
    if (closed && referencedID?.toLowerCase() === normalizedWhiteboardID) {
      removedCount += 1;
      continue;
    }

    keptLines.push(...blockLines);
  }

  return {
    markdown: removedCount
      ? normalizeRemovedReferenceSpacing(keptLines.join("\n"))
      : markdown,
    removedCount,
  };
}

function escapeHtml(value: string): string {
  return value.replace(/[&<>"']/g, (char) => {
    const entities: Record<string, string> = {
      "&": "&amp;",
      "<": "&lt;",
      ">": "&gt;",
      '"': "&quot;",
      "'": "&#039;",
    };
    return entities[char] || char;
  });
}

export function createWhiteboardPlaceholder(whiteboardID: string): string {
  const escapedID = escapeHtml(whiteboardID);

  return [
    `<button class="whiteboard-reference" type="button" data-whiteboard-reference="true" data-whiteboard-id="${escapedID}">`,
    '<span class="whiteboard-reference__label">Excalidraw</span>',
    '<span class="whiteboard-reference__title">Whiteboard</span>',
    `<span class="whiteboard-reference__id">${escapedID}</span>`,
    "</button>",
  ].join("");
}

export function createWhiteboardPreviewMarkup(source: string): string {
  const whiteboardID = extractWhiteboardID(source);
  if (!whiteboardID) {
    return '<pre class="diagram-error">Invalid Excalidraw whiteboard reference</pre>';
  }
  return createWhiteboardPlaceholder(whiteboardID);
}

export interface WhiteboardRuntimeOptions {
  contentKey?: string;
  onOpen?: (whiteboardID: string) => void;
}

export function whiteboardRuntime(
  node: HTMLElement,
  initialOptions: WhiteboardRuntimeOptions,
) {
  let options = initialOptions;

  function handleClick(event: MouseEvent): void {
    const target = event.target;
    if (!(target instanceof Element)) return;

    const reference = target.closest<HTMLElement>("[data-whiteboard-reference='true']");
    const whiteboardID = reference?.dataset.whiteboardId;
    if (!whiteboardID) return;

    event.preventDefault();
    options.onOpen?.(whiteboardID);
  }

  node.addEventListener("click", handleClick);

  return {
    update(nextOptions: WhiteboardRuntimeOptions): void {
      options = nextOptions;
    },
    destroy(): void {
      node.removeEventListener("click", handleClick);
    },
  };
}
