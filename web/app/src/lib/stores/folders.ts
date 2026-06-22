import { get, writable } from "svelte/store";
import {
  createFolder,
  deleteFolder,
  getFolderSettings,
  listFolders,
  updateFolder as updateFolderAPI,
  updateFolderSettings,
  type CreateFolderInput,
  type UpdateFolderInput,
} from "../api/folders";
import type { Folder, FolderSettings, UUID } from "../api/types";
import { t } from "./preferences";

interface FoldersState {
  folders: Folder[];
  activeFolderID: UUID | null;
  expandedFolderIDs: UUID[];
  settings: FolderSettings;
  loading: boolean;
  error: string;
}

export interface FolderTreeItem {
  folder: Folder;
  depth: number;
  children: FolderTreeItem[];
}

export interface FolderOption {
  id: UUID;
  name: string;
  depth: number;
  disabled?: boolean;
}

const defaultSettings: FolderSettings = { max_depth: 3 };

export function buildFolderTree(folders: Folder[]): FolderTreeItem[] {
  const childrenByParent = new Map<string, Folder[]>();
  const roots: Folder[] = [];

  for (const folder of folders) {
    const parentID = folder.parent_id ?? "";
    if (!folder.parent_id) {
      roots.push(folder);
      continue;
    }
    const children = childrenByParent.get(parentID) ?? [];
    children.push(folder);
    childrenByParent.set(parentID, children);
  }

  const sortFolders = (items: Folder[]) =>
    [...items].sort((left, right) => {
      if (left.sort_order !== right.sort_order) return left.sort_order - right.sort_order;
      return left.name.localeCompare(right.name);
    });

  const build = (items: Folder[], depth: number): FolderTreeItem[] =>
    sortFolders(items).map((folder) => ({
      folder,
      depth,
      children: build(childrenByParent.get(folder.id) ?? [], depth + 1),
    }));

  return build(roots, 1);
}

export function flattenFolderTree(tree: FolderTreeItem[]): FolderOption[] {
  const result: FolderOption[] = [];
  const visit = (items: FolderTreeItem[]) => {
    for (const item of items) {
      result.push({
        id: item.folder.id,
        name: item.folder.name,
        depth: item.depth,
      });
      visit(item.children);
    }
  };
  visit(tree);
  return result;
}

function createFoldersStore() {
  const { subscribe, update } = writable<FoldersState>({
    folders: [],
    activeFolderID: null,
    expandedFolderIDs: [],
    settings: defaultSettings,
    loading: false,
    error: "",
  });

  function expandAncestors(folderID: UUID | null, folders: Folder[]): UUID[] {
    if (!folderID) return [];
    const byID = new Map(folders.map((folder) => [folder.id, folder]));
    const expanded: UUID[] = [];
    let current = byID.get(folderID);
    while (current?.parent_id) {
      expanded.push(current.parent_id);
      current = byID.get(current.parent_id);
    }
    return expanded;
  }

  function mergeExpanded(current: UUID[], additions: UUID[]): UUID[] {
    const merged = new Set(current);
    for (const id of additions) merged.add(id);
    return [...merged];
  }

  return {
    subscribe,
    async load() {
      update((state) => ({ ...state, loading: true, error: "" }));
      try {
        const [folders, settings] = await Promise.all([
          listFolders(),
          getFolderSettings(),
        ]);
        update((state) => ({
          ...state,
          folders,
          settings,
          expandedFolderIDs: mergeExpanded(
            state.expandedFolderIDs,
            expandAncestors(state.activeFolderID, folders),
          ),
          loading: false,
          error: "",
        }));
      } catch (error) {
        update((state) => ({
          ...state,
          loading: false,
          error:
            error instanceof Error
              ? error.message
              : t("folderLoadFailed"),
        }));
      }
    },
    select(folderID: UUID | null) {
      update((state) => ({
        ...state,
        activeFolderID: folderID,
        expandedFolderIDs: mergeExpanded(
          state.expandedFolderIDs,
          expandAncestors(folderID, state.folders),
        ),
      }));
    },
    toggleExpanded(folderID: UUID) {
      update((state) => ({
        ...state,
        expandedFolderIDs: state.expandedFolderIDs.includes(folderID)
          ? state.expandedFolderIDs.filter((id) => id !== folderID)
          : [...state.expandedFolderIDs, folderID],
      }));
    },
    async create(input: CreateFolderInput): Promise<Folder> {
      const folder = await createFolder(input);
      update((state) => ({
        ...state,
        folders: [...state.folders, folder],
        expandedFolderIDs: input.parent_id
          ? mergeExpanded(state.expandedFolderIDs, [input.parent_id])
          : state.expandedFolderIDs,
      }));
      return folder;
    },
    async updateFolder(folderID: UUID, input: UpdateFolderInput): Promise<Folder> {
      const folder = await updateFolderAPI(folderID, input);
      update((state) => ({
        ...state,
        folders: state.folders.map((item) => (item.id === folder.id ? folder : item)),
        expandedFolderIDs:
          input.parent_id === null
            ? state.expandedFolderIDs
            : input.parent_id
              ? mergeExpanded(state.expandedFolderIDs, [input.parent_id])
              : state.expandedFolderIDs,
      }));
      return folder;
    },
    async delete(folderID: UUID): Promise<void> {
      await deleteFolder(folderID);
      update((state) => ({
        ...state,
        folders: state.folders.filter((folder) => folder.id !== folderID),
        activeFolderID:
          state.activeFolderID === folderID ? null : state.activeFolderID,
        expandedFolderIDs: state.expandedFolderIDs.filter((id) => id !== folderID),
      }));
    },
    async saveSettings(settings: FolderSettings): Promise<FolderSettings> {
      const saved = await updateFolderSettings(settings);
      update((state) => ({ ...state, settings: saved }));
      return saved;
    },
    tree(): FolderTreeItem[] {
      return buildFolderTree(get({ subscribe }).folders);
    },
    options(): FolderOption[] {
      return flattenFolderTree(buildFolderTree(get({ subscribe }).folders));
    },
  };
}

export const foldersStore = createFoldersStore();
