import { API_BASE, ApiError, apiFetch, getToken } from "./client";

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
  onProgress?: (progress: number) => void;
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

  return new Promise((resolve, reject) => {
    const request = new XMLHttpRequest();
    request.open("POST", `${API_BASE}/fonts`);

    const token = getToken();
    if (token) request.setRequestHeader("Authorization", `Bearer ${token}`);

    request.upload.onprogress = (event) => {
      if (!event.lengthComputable) return;
      options.onProgress?.(
        Math.min(99, Math.round((event.loaded / event.total) * 100)),
      );
    };

    request.onload = () => {
      let payload: unknown = null;
      if (request.responseText) {
        try {
          payload = JSON.parse(request.responseText);
        } catch {
          reject(new Error("Font upload returned an invalid response"));
          return;
        }
      }

      if (request.status < 200 || request.status >= 300) {
        const message =
          payload &&
          typeof payload === "object" &&
          "error" in payload &&
          typeof payload.error === "string"
            ? payload.error
            : `Request failed: ${request.status}`;
        reject(new ApiError(message, request.status, payload));
        return;
      }

      options.onProgress?.(100);
      resolve(payload as FontRecord);
    };

    request.onerror = () => reject(new Error("Font upload failed"));
    request.onabort = () => reject(new Error("Font upload canceled"));
    request.send(form);
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
