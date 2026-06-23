import { apiFetch } from "./client";

export type BackupTargetType = "webdav" | "s3";
export type BackupSchedule = "manual" | "daily" | "weekly" | "monthly";
export type BackupStatus = "never" | "success" | "failed";

export interface BackupTarget {
  id: number;
  name: string;
  type: BackupTargetType;
  enabled: boolean;
  webdav_url?: string;
  webdav_user?: string;
  has_webdav_password: boolean;
  s3_endpoint?: string;
  s3_region?: string;
  s3_bucket?: string;
  has_s3_access_key: boolean;
  has_s3_secret_key: boolean;
  last_backup_status: BackupStatus;
  last_backup_error?: string;
  last_backup_at?: string;
  last_test_status: BackupStatus;
  last_test_error?: string;
  last_test_at?: string;
  created_at: string;
  updated_at: string;
}

export interface BackupTargetInput {
  name: string;
  type: BackupTargetType;
  enabled: boolean;
  webdav_url?: string;
  webdav_user?: string;
  webdav_password?: string;
  s3_endpoint?: string;
  s3_region?: string;
  s3_bucket?: string;
  s3_access_key?: string;
  s3_secret_key?: string;
}

export interface BackupTask {
  id: number;
  name: string;
  enabled: boolean;
  schedule: BackupSchedule;
  retention_days: number;
  max_count: number;
  target_ids: number[];
  targets: BackupTarget[];
  last_backup_status: BackupStatus;
  last_backup_error?: string;
  last_backup_at?: string;
  next_run_at?: string;
  created_at: string;
  updated_at: string;
}

export interface BackupTaskInput {
  name: string;
  enabled: boolean;
  schedule: BackupSchedule;
  retention_days: number;
  max_count: number;
  target_ids: number[];
}

export interface BackupConnectionTestResponse {
  ok: boolean;
  message: string;
  checked_at: string;
}

export interface BackupFileInfo {
  filename: string;
  size: number;
  created_at: string;
}

export interface BackupListResponse {
  backups: BackupFileInfo[] | null;
}

export interface BackupTargetRunResult {
  target_id: number;
  name: string;
  type: BackupTargetType;
  ok: boolean;
  error?: string;
}

export interface BackupRunResponse {
  message: string;
  file: string;
  results: BackupTargetRunResult[];
}

export interface BackupRestoreResponse {
  message: string;
  warning?: string;
  restart_required?: boolean;
}

export interface FileCheckResult {
  path: string;
  exists: boolean;
  size: number;
  is_dir: boolean;
  error?: string;
}

export interface BackupVerificationResult {
  valid: boolean;
  error?: string;
  file_checks: FileCheckResult[];
  total_size: number;
  file_count: number;
  verified_at: string;
}

export function listBackupTargets(): Promise<BackupTarget[]> {
  return apiFetch<BackupTarget[]>("/backup/targets");
}

export function createBackupTarget(
  target: BackupTargetInput,
): Promise<BackupTarget> {
  return apiFetch<BackupTarget>("/backup/targets", {
    method: "POST",
    body: JSON.stringify(target),
  });
}

export function updateBackupTarget(
  id: number,
  target: BackupTargetInput,
): Promise<BackupTarget> {
  return apiFetch<BackupTarget>(`/backup/targets/${id}`, {
    method: "PUT",
    body: JSON.stringify(target),
  });
}

export function deleteBackupTarget(id: number): Promise<void> {
  return apiFetch<void>(`/backup/targets/${id}`, {
    method: "DELETE",
  });
}

export function testUnsavedBackupTarget(
  target: BackupTargetInput,
): Promise<BackupConnectionTestResponse> {
  return apiFetch<BackupConnectionTestResponse>("/backup/targets/test", {
    method: "POST",
    body: JSON.stringify(target),
  });
}

export function testBackupTarget(
  id: number,
  target?: BackupTargetInput,
): Promise<BackupConnectionTestResponse> {
  return apiFetch<BackupConnectionTestResponse>(`/backup/targets/${id}/test`, {
    method: "POST",
    body: target ? JSON.stringify(target) : undefined,
  });
}

export function listBackupTasks(): Promise<BackupTask[]> {
  return apiFetch<BackupTask[]>("/backup/tasks");
}

export function createBackupTask(task: BackupTaskInput): Promise<BackupTask> {
  return apiFetch<BackupTask>("/backup/tasks", {
    method: "POST",
    body: JSON.stringify(task),
  });
}

export function updateBackupTask(
  id: number,
  task: BackupTaskInput,
): Promise<BackupTask> {
  return apiFetch<BackupTask>(`/backup/tasks/${id}`, {
    method: "PUT",
    body: JSON.stringify(task),
  });
}

export function deleteBackupTask(id: number): Promise<void> {
  return apiFetch<void>(`/backup/tasks/${id}`, {
    method: "DELETE",
  });
}

export function runBackupTask(id: number): Promise<BackupRunResponse> {
  return apiFetch<BackupRunResponse>(`/backup/tasks/${id}/run`, {
    method: "POST",
  });
}

export function listBackupFiles(
  targetId: number,
): Promise<BackupListResponse> {
  return apiFetch<BackupListResponse>(`/backup/targets/${targetId}/files`);
}

export function verifyBackupFile(
  targetId: number,
  filename: string,
): Promise<BackupVerificationResult> {
  return apiFetch<BackupVerificationResult>(
    `/backup/targets/${targetId}/verify`,
    {
      method: "POST",
      body: JSON.stringify({ filename }),
    },
  );
}

export function restoreBackupFile(
  targetId: number,
  filename: string,
  confirmation: string,
): Promise<BackupRestoreResponse> {
  return apiFetch<BackupRestoreResponse>(
    `/backup/targets/${targetId}/restore`,
    {
      method: "POST",
      body: JSON.stringify({ filename, confirmation }),
    },
  );
}
