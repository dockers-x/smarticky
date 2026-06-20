import { apiFetch } from "./client";
import type { User } from "./types";

export interface ManagedUser extends User {
  created_at?: string;
}

export interface CreateUserPayload {
  username: string;
  password: string;
  email?: string;
  nickname?: string;
  role: "admin" | "user";
}

export interface UpdateUserPayload {
  email?: string;
  nickname?: string;
  avatar?: string;
  role?: "admin" | "user";
}

export interface UpdatePasswordPayload {
  old_password: string;
  new_password: string;
}

export function listUsers(): Promise<ManagedUser[]> {
  return apiFetch<ManagedUser[]>("/users");
}

export function createUser(payload: CreateUserPayload): Promise<ManagedUser> {
  return apiFetch<ManagedUser>("/users", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export function updateUser(
  userID: number,
  payload: UpdateUserPayload,
): Promise<User> {
  return apiFetch<User>(`/users/${userID}`, {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}

export function updatePassword(
  userID: number,
  payload: UpdatePasswordPayload,
): Promise<{ message: string }> {
  return apiFetch<{ message: string }>(`/users/${userID}/password`, {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}

export function uploadAvatar(
  userID: number,
  file: File,
): Promise<{ avatar: string }> {
  const form = new FormData();
  form.set("avatar", file);
  return apiFetch<{ avatar: string }>(`/users/${userID}/avatar`, {
    method: "POST",
    body: form,
  });
}

export function deleteUser(userID: number): Promise<{ message: string }> {
  return apiFetch<{ message: string }>(`/users/${userID}`, {
    method: "DELETE",
  });
}
