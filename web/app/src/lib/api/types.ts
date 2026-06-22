export type UUID = string;

export interface User {
  id: number;
  username: string;
  email?: string;
  nickname?: string;
  role: "admin" | "user";
  avatar?: string;
  share_signature?: string;
  time_zone?: string;
  lazycat_uid?: string | null;
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
  download_url?: string;
}

export interface Whiteboard {
  id: UUID;
  note_id: UUID;
  title: string;
  scene_json: string;
  thumbnail?: string;
  created_at: string;
  updated_at: string;
}

export interface ExcalidrawLibrary {
  id: UUID;
  library_json: string;
  created_at: string;
  updated_at: string;
}

export type ProtectionMode = "none" | "password" | "encrypted";

export interface Note {
  id: UUID;
  title: string;
  content: string;
  color: string;
  protection_mode: ProtectionMode;
  content_redacted: boolean;
  encrypted_content?: string;
  encryption_alg?: string;
  encryption_kdf?: string;
  encryption_salt?: string;
  encryption_nonce?: string;
  is_starred: boolean;
  is_deleted: boolean;
  folder_id?: UUID | null;
  tags?: Tag[];
  created_at: string;
  updated_at: string;
}

export interface NoteMetadata {
  id: UUID;
  title: string;
  color: string;
  protection_mode: ProtectionMode;
  content_redacted: boolean;
  is_starred: boolean;
  is_deleted: boolean;
  folder_id?: UUID | null;
  created_at: string;
  updated_at: string;
}

export interface NoteLinkGraphEdge {
  id: UUID;
  source: UUID;
  target: UUID;
  link_type: string;
  display_text: string;
  occurrence_count: number;
}

export interface NoteLinkGraph {
  nodes: NoteMetadata[];
  edges: NoteLinkGraphEdge[];
}

export interface Folder {
  id: UUID;
  name: string;
  parent_id?: UUID | null;
  sort_order: number;
  is_starred: boolean;
  note_count: number;
  child_count: number;
  depth: number;
  created_at: string;
  updated_at: string;
}

export interface FolderSettings {
  max_depth: number;
}

export interface SetupCheckResponse {
  setup_needed: boolean;
}
