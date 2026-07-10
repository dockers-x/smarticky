import { writable } from "svelte/store";
import { apiFetch } from "../api/client";
import type { Tag, UUID } from "../api/types";

export const allTags = writable<Tag[]>([]);
let tagLoadPromise: Promise<void> | null = null;

export async function loadTags(): Promise<void> {
  if (tagLoadPromise) return tagLoadPromise;

  const request = apiFetch<Tag[]>("/tags").then((tags) => {
    allTags.set(tags);
  });
  tagLoadPromise = request.finally(() => {
    tagLoadPromise = null;
  });
  return tagLoadPromise;
}

export async function addToNote(noteId: UUID, tagName: string): Promise<void> {
  const trimmed = tagName.trim();
  if (!trimmed) return;

  const existing = (await apiFetch<Tag[]>("/tags")).find(
    (tag) => tag.name.toLowerCase() === trimmed.toLowerCase(),
  );
  const tag =
    existing ||
    (await apiFetch<Tag>("/tags", {
      method: "POST",
      body: JSON.stringify({ name: trimmed, color: "#E8450A" }),
    }));

  await apiFetch(`/notes/${noteId}/tags/${tag.id}`, { method: "POST" });
  await loadTags();
}

export async function removeFromNote(noteId: UUID, tagId: UUID): Promise<void> {
  await apiFetch(`/notes/${noteId}/tags/${tagId}`, { method: "DELETE" });
  await loadTags();
}

export const tagsStore = {
  subscribe: allTags.subscribe,
  load: loadTags,
  addToNote,
  removeFromNote,
};
