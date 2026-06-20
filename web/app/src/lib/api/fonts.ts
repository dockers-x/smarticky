import { apiFetch, getToken } from "./client";

export interface FontRecord {
  id: string;
  name: string;
  display_name: string;
  format: "ttf" | "otf" | "woff" | "woff2";
  file_size: number;
  preview_text: string;
  is_shared: boolean;
  uploaded_by: string;
  uploader_id: number;
  download_url: string;
  created_at: string;
}

export interface UploadFontOptions {
  file: File;
  displayName: string;
  isShared: boolean;
  previewText?: string;
}

export function listFonts(): Promise<FontRecord[]> {
  return apiFetch<FontRecord[]>("/fonts");
}

export function uploadFont(options: UploadFontOptions): Promise<FontRecord> {
  const form = new FormData();
  form.set("file", options.file);
  form.set("display_name", options.displayName);
  form.set("is_shared", options.isShared ? "true" : "false");
  if (options.previewText) {
    form.set("preview_text", options.previewText);
  }

  return apiFetch<FontRecord>("/fonts", {
    method: "POST",
    body: form,
  });
}

export function deleteFont(fontID: string): Promise<void> {
  return apiFetch<void>(`/fonts/${fontID}`, { method: "DELETE" });
}

export async function downloadFontBlob(font: FontRecord): Promise<Blob> {
  const headers = new Headers();
  const token = getToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);

  const response = await fetch(font.download_url, { headers });
  if (!response.ok) {
    throw new Error(`Font download failed: ${response.status}`);
  }

  return response.blob();
}
