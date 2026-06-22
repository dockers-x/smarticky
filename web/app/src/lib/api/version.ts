import { apiFetch } from "./client";

export interface VersionInfo {
  version: string;
  build_time: string;
  git_commit: string;
}

export async function getVersionInfo(): Promise<VersionInfo> {
  return apiFetch<VersionInfo>("/version");
}
