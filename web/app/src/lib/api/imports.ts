import { apiFetch } from "./client";

export interface ImportPreview {
  job_id: number;
  filename: string;
  note_count: number;
  tag_count: number;
  resource_count: number;
  warning_count: number;
}

export interface ImportResult {
  job_id: number;
  status: "completed" | "completed_with_errors" | "failed";
  imported_count: number;
  skipped_count: number;
  failed_count: number;
}

export async function previewEvernote(file: File): Promise<ImportPreview> {
  const form = new FormData();
  form.set("file", file);
  return apiFetch<ImportPreview>("/import/evernote/preview", {
    method: "POST",
    body: form,
  });
}

export async function confirmEvernote(jobId: number): Promise<ImportResult> {
  return apiFetch<ImportResult>("/import/evernote/confirm", {
    method: "POST",
    body: JSON.stringify({ job_id: jobId }),
  });
}
