import { apiFetch } from "./client";
import type { NoteLinkGraph } from "./types";

export function fetchNoteLinkGraph(includeTrash = false): Promise<NoteLinkGraph> {
  const query = includeTrash ? "?include_trash=true" : "";
  return apiFetch<NoteLinkGraph>(`/note-links${query}`);
}
