import { apiFetch } from "./client";
import type { ExcalidrawLibrary } from "./types";

export interface UpdateExcalidrawLibraryInput {
  library_json: string;
}

export function getExcalidrawLibrary(): Promise<ExcalidrawLibrary> {
  return apiFetch<ExcalidrawLibrary>("/excalidraw/library");
}

export function updateExcalidrawLibrary(
  input: UpdateExcalidrawLibraryInput,
): Promise<ExcalidrawLibrary> {
  return apiFetch<ExcalidrawLibrary>("/excalidraw/library", {
    method: "PUT",
    body: JSON.stringify(input),
  });
}
