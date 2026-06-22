import { apiFetch } from "./client";
import type { UUID, Whiteboard } from "./types";

export interface CreateWhiteboardInput {
  title?: string;
  scene_json?: string;
  thumbnail?: string;
}

export interface UpdateWhiteboardInput {
  title?: string;
  scene_json?: string;
  thumbnail?: string;
}

export function listWhiteboards(noteID: UUID): Promise<Whiteboard[]> {
  return apiFetch<Whiteboard[]>(`/notes/${noteID}/whiteboards`);
}

export function createWhiteboard(
  noteID: UUID,
  input: CreateWhiteboardInput = {},
): Promise<Whiteboard> {
  return apiFetch<Whiteboard>(`/notes/${noteID}/whiteboards`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function getWhiteboard(whiteboardID: UUID): Promise<Whiteboard> {
  return apiFetch<Whiteboard>(`/whiteboards/${whiteboardID}`);
}

export function updateWhiteboard(
  whiteboardID: UUID,
  input: UpdateWhiteboardInput,
): Promise<Whiteboard> {
  return apiFetch<Whiteboard>(`/whiteboards/${whiteboardID}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function deleteWhiteboard(whiteboardID: UUID): Promise<void> {
  return apiFetch<void>(`/whiteboards/${whiteboardID}`, { method: "DELETE" });
}
