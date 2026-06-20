export type UUID = string;

export interface User {
  id: number;
  username: string;
  email?: string;
  nickname?: string;
  role: "admin" | "user";
  avatar?: string;
}

export interface Tag {
  id: UUID;
  name: string;
  color: string;
  created_at?: string;
  updated_at?: string;
}

export interface Attachment {
  id: number;
  filename: string;
  file_size: number;
  mime_type?: string;
  created_at: string;
}

export interface Note {
  id: UUID;
  title: string;
  content: string;
  color: string;
  is_locked: boolean;
  is_starred: boolean;
  is_deleted: boolean;
  tags?: Tag[];
  created_at: string;
  updated_at: string;
}

export interface SetupCheckResponse {
  setup_needed: boolean;
}
