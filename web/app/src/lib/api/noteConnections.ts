import { apiFetch } from "./client";

export type NoteConnectionProvider = "siyuan" | "notion" | "joplin";
export type NoteConnectionStatus = "never" | "success" | "failed";

export interface NoteConnectionAccount {
  id: number;
  name: string;
  provider: NoteConnectionProvider;
  endpoint: string;
  enabled: boolean;
  auth_type: "token";
  has_credentials: boolean;
  default_target_id: string;
  default_target_name: string;
  last_test_status: NoteConnectionStatus;
  last_test_error?: string;
  last_test_at?: string;
  created_at: string;
  updated_at: string;
}

export interface NoteConnectionAccountInput {
  name: string;
  provider: NoteConnectionProvider;
  endpoint: string;
  token?: string;
  default_target_id: string;
  default_target_name: string;
  enabled: boolean;
  clear_credentials?: boolean;
}

export interface NoteConnectionTarget {
  id: string;
  name: string;
  kind: string;
  parent_id?: string;
}

export interface NoteConnectionJob {
  id: number;
  provider: NoteConnectionProvider;
  operation: "import" | "push";
  status: "pending" | "running" | "completed" | "completed_with_errors" | "failed";
  total_count: number;
  imported_count: number;
  pushed_count: number;
  skipped_count: number;
  failed_count: number;
  message?: string;
  created_at: string;
  completed_at?: string;
}

export interface NoteConnectionImportResult {
  job_id: number;
  status: string;
  total_count: number;
  imported_count: number;
  skipped_count: number;
  failed_count: number;
}

export interface NoteConnectionPushResult {
  job_id: number;
  status: string;
  result: {
    external_id: string;
    target_id?: string;
    path?: string;
    url?: string;
  };
  finished_at: string;
}

export async function listNoteConnectionAccounts(): Promise<NoteConnectionAccount[]> {
  return apiFetch<NoteConnectionAccount[]>("/note-connections/accounts");
}

export async function createNoteConnectionAccount(
  input: NoteConnectionAccountInput,
): Promise<NoteConnectionAccount> {
  return apiFetch<NoteConnectionAccount>("/note-connections/accounts", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateNoteConnectionAccount(
  id: number,
  input: NoteConnectionAccountInput,
): Promise<NoteConnectionAccount> {
  return apiFetch<NoteConnectionAccount>(`/note-connections/accounts/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteNoteConnectionAccount(id: number): Promise<void> {
  return apiFetch<void>(`/note-connections/accounts/${id}`, { method: "DELETE" });
}

export async function testUnsavedNoteConnectionAccount(
  input: NoteConnectionAccountInput,
): Promise<{ status: NoteConnectionStatus; error?: string }> {
  return apiFetch<{ status: NoteConnectionStatus; error?: string }>("/note-connections/accounts/test", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function testNoteConnectionAccount(
  id: number,
  token?: string,
): Promise<{ status: NoteConnectionStatus; account?: NoteConnectionAccount; error?: string }> {
  return apiFetch<{ status: NoteConnectionStatus; account?: NoteConnectionAccount; error?: string }>(
    `/note-connections/accounts/${id}/test`,
    {
      method: "POST",
      body: JSON.stringify(token ? { token } : {}),
    },
  );
}

export async function listNoteConnectionTargets(id: number): Promise<NoteConnectionTarget[]> {
  return apiFetch<NoteConnectionTarget[]>(`/note-connections/accounts/${id}/targets`);
}

export async function importFromNoteConnection(
  id: number,
  targetId: string,
  limit: number,
): Promise<NoteConnectionImportResult> {
  return apiFetch<NoteConnectionImportResult>(`/note-connections/accounts/${id}/import`, {
    method: "POST",
    body: JSON.stringify({ target_id: targetId, limit }),
  });
}

export async function pushNoteToConnection(
  id: number,
  noteId: string,
  targetId: string,
): Promise<NoteConnectionPushResult> {
  return apiFetch<NoteConnectionPushResult>(`/note-connections/accounts/${id}/push`, {
    method: "POST",
    body: JSON.stringify({ note_id: noteId, target_id: targetId }),
  });
}

export async function listNoteConnectionJobs(): Promise<NoteConnectionJob[]> {
  return apiFetch<NoteConnectionJob[]>("/note-connections/jobs");
}
