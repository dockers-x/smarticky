import { apiFetch } from "./client";
import type { Folder, FolderSettings, UUID } from "./types";

export interface CreateFolderInput {
  name: string;
  parent_id?: UUID | null;
  sort_order?: number;
}

export interface UpdateFolderInput {
  name?: string;
  parent_id?: UUID | null;
  sort_order?: number;
  is_starred?: boolean;
}

export function listFolders(): Promise<Folder[]> {
  return apiFetch<Folder[]>("/folders");
}

export function createFolder(input: CreateFolderInput): Promise<Folder> {
  return apiFetch<Folder>("/folders", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateFolder(
  folderID: UUID,
  input: UpdateFolderInput,
): Promise<Folder> {
  return apiFetch<Folder>(`/folders/${folderID}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function deleteFolder(folderID: UUID): Promise<void> {
  return apiFetch<void>(`/folders/${folderID}`, { method: "DELETE" });
}

export function getFolderSettings(): Promise<FolderSettings> {
  return apiFetch<FolderSettings>("/folders/settings");
}

export function updateFolderSettings(
  settings: FolderSettings,
): Promise<FolderSettings> {
  return apiFetch<FolderSettings>("/folders/settings", {
    method: "PUT",
    body: JSON.stringify(settings),
  });
}
