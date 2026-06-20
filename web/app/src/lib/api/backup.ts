import { apiFetch } from "./client";

export type BackupBackend = "webdav" | "s3";
export type BackupSchedule = "daily" | "weekly" | "manual";

export interface BackupConfig {
  id?: number;
  webdav_url?: string;
  webdav_user?: string;
  webdav_password?: string;
  s3_endpoint?: string;
  s3_region?: string;
  s3_bucket?: string;
  s3_access_key?: string;
  s3_secret_key?: string;
  auto_backup_enabled: boolean;
  backup_schedule: BackupSchedule;
  backup_retention_days: number;
  backup_max_count: number;
  last_backup_at?: string;
  created_at?: string;
  updated_at?: string;
}

export interface BackupFileInfo {
  filename: string;
  size: number;
  created_at: string;
}

export interface BackupListResponse {
  backups: BackupFileInfo[] | null;
}

export interface BackupRunResponse {
  message: string;
  file: string;
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

export function getBackupConfig(): Promise<BackupConfig> {
  return apiFetch<BackupConfig>("/backup/config");
}

export function updateBackupConfig(
  config: Partial<BackupConfig>,
): Promise<BackupConfig> {
  return apiFetch<BackupConfig>("/backup/config", {
    method: "PUT",
    body: JSON.stringify(config),
  });
}

export function runBackup(backend: BackupBackend): Promise<BackupRunResponse> {
  return apiFetch<BackupRunResponse>(`/backup/${backend}`, {
    method: "POST",
  });
}

export function listBackups(
  backend: BackupBackend,
): Promise<BackupListResponse> {
  return apiFetch<BackupListResponse>(`/backup/list/${backend}`);
}

export function verifyBackup(
  backend: BackupBackend,
  filename: string,
): Promise<BackupVerificationResult> {
  return apiFetch<BackupVerificationResult>(`/backup/verify/${backend}`, {
    method: "POST",
    body: JSON.stringify({ filename }),
  });
}

export function restoreBackup(
  backend: BackupBackend,
  filename: string,
): Promise<BackupRestoreResponse> {
  return apiFetch<BackupRestoreResponse>(`/restore/${backend}`, {
    method: "POST",
    body: JSON.stringify({ filename }),
  });
}
