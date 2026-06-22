import { writable } from "svelte/store";
import { apiFetch } from "../api/client";
import type { Attachment, UUID } from "../api/types";

export const attachments = writable<Attachment[]>([]);
let loadSequence = 0;

export async function loadAttachments(noteId: UUID): Promise<void> {
  const sequence = ++loadSequence;
  const nextAttachments = await apiFetch<Attachment[] | null>(
    `/notes/${noteId}/attachments`,
  );

  if (sequence === loadSequence) {
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
  await loadAttachments(noteId);
  return attachment;
}

export const attachmentsStore = {
  subscribe: attachments.subscribe,
  load: loadAttachments,
  upload: uploadAttachment,
};
