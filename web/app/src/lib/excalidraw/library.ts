import {
  getExcalidrawLibrary,
  updateExcalidrawLibrary,
} from "../api/excalidrawLibrary";

const addLibraryParam = "addLibrary";
const libraryTokenParam = "token";
const whiteboardParam = "whiteboard";

function isAllowedLibraryURL(rawURL: string): boolean {
  try {
    const url = new URL(rawURL);
    return (
      url.protocol === "https:" &&
      (url.hostname === "libraries.excalidraw.com" ||
        url.hostname.endsWith(".excalidraw.com"))
    );
  } catch {
    return false;
  }
}

export function parseExcalidrawLibraryJSON(libraryJSON: string): readonly unknown[] {
  try {
    const parsed = JSON.parse(libraryJSON);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

export function serializeExcalidrawLibrary(libraryItems: readonly unknown[]): string {
  return JSON.stringify(Array.isArray(libraryItems) ? libraryItems : []);
}

export function excalidrawLibraryReturnURL(whiteboardID?: string): string {
  if (typeof window === "undefined") return "";
  const query = new URLSearchParams();
  if (whiteboardID) {
    query.set(whiteboardParam, whiteboardID);
  }
  const queryString = query.toString();
  return `${window.location.origin}${window.location.pathname}${
    queryString ? `?${queryString}` : ""
  }`;
}

export function whiteboardIDFromLibraryCallback(): string {
  if (typeof window === "undefined") return "";
  const query = new URLSearchParams(window.location.search);
  const queryWhiteboardID = query.get(whiteboardParam);
  if (queryWhiteboardID) return queryWhiteboardID;

  const hash = new URLSearchParams(window.location.hash.slice(1));
  return hash.get(whiteboardParam) || "";
}

function libraryURLFromHash(): string | null {
  if (typeof window === "undefined") return null;
  const hash = new URLSearchParams(window.location.hash.slice(1));
  return hash.get(addLibraryParam);
}

function clearLibraryCallbackHash(): void {
  if (typeof window === "undefined") return;

  const hash = new URLSearchParams(window.location.hash.slice(1));
  hash.delete(addLibraryParam);
  hash.delete(libraryTokenParam);
  hash.delete(whiteboardParam);
  const query = new URLSearchParams(window.location.search);
  query.delete(whiteboardParam);

  const nextHash = hash.toString();
  const nextQuery = query.toString();
  const nextURL = `${window.location.pathname}${nextQuery ? `?${nextQuery}` : ""}${
    nextHash ? `#${nextHash}` : ""
  }`;
  window.history.replaceState({}, "", nextURL);
}

export interface ExcalidrawLibraryImportResult {
  imported: boolean;
  whiteboardID: string;
}

export async function importExcalidrawLibraryFromCallback(): Promise<ExcalidrawLibraryImportResult> {
  const whiteboardID = whiteboardIDFromLibraryCallback();
  const libraryURL = libraryURLFromHash();
  if (!libraryURL) return { imported: false, whiteboardID };

  try {
    if (!isAllowedLibraryURL(libraryURL)) {
      throw new Error("Unsupported Excalidraw library URL");
    }

    const response = await fetch(libraryURL, {
      credentials: "omit",
    });
    if (!response.ok) {
      throw new Error(`Library download failed: ${response.status}`);
    }

    const [blob, currentLibrary, excalidraw] = await Promise.all([
      response.blob(),
      getExcalidrawLibrary(),
      import("@excalidraw/excalidraw"),
    ]);
    const importedItems = await excalidraw.loadLibraryFromBlob(blob, "published");
    const currentItems = parseExcalidrawLibraryJSON(
      currentLibrary.library_json,
    ) as Parameters<typeof excalidraw.mergeLibraryItems>[0];
    const mergedItems = excalidraw.mergeLibraryItems(
      currentItems,
      importedItems,
    );

    await updateExcalidrawLibrary({
      library_json: serializeExcalidrawLibrary(mergedItems),
    });
    return { imported: true, whiteboardID };
  } finally {
    clearLibraryCallbackHash();
  }
}
