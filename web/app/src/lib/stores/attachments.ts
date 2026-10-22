import { writable } from "svelte/store";
import { apiFetch } from "../api/client";
import type { Attachment, UUID } from "../api/types";

export const attachments = writable<Attachment[]>([]);
let loadSequence = 0;
let currentNoteId: UUID | null = null;

export async function loadAttachments(noteId: UUID): Promise<void> {
  currentNoteId = noteId;
  const sequence = ++loadSequence;
  const nextAttachments = await apiFetch<Attachment[] | null>(
    `/notes/${noteId}/attachments`,
  );

  if (sequence === loadSequence && currentNoteId === noteId) {
    attachments.set(nextAttachments ?? []);
  }
}

export async function uploadAttachment(
  noteId: UUID,
  file: File,
): Promise<Attachment> {
  const form = new FormData();
  form.set("file", file);
  const attachment = await apiFetch<Attachment>(`/notes/${noteId}/attachments`, {
    method: "POST",
    body: form,
  });
  if (currentNoteId === noteId) {
    await loadAttachments(noteId).catch(() => undefined);
  }
  return attachment;
}

export const attachmentsStore = {
  subscribe: attachments.subscribe,
  load: loadAttachments,
  upload: uploadAttachment,
};
