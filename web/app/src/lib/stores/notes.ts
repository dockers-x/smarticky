import { get, writable } from "svelte/store";
import { apiFetch } from "../api/client";
import type { Note, ProtectionMode } from "../api/types";
import { preferencesStore, t } from "./preferences";

export type NoteFilter = "all" | "starred" | "trash";
export type WorkspaceView = "notes" | "index";

export interface NoteSearchFilters {
  title: string;
  tags: string[];
  createdFrom: string;
  createdTo: string;
  updatedFrom: string;
  updatedTo: string;
}

interface NotesState {
  notes: Note[];
  selected: Note | null;
  workspaceView: WorkspaceView;
  filter: NoteFilter;
  folderID: string | null;
  folderBrowserOpen: boolean;
  search: string;
  searchFilters: NoteSearchFilters;
  loading: boolean;
  error: string;
}

type NoteUpdateFields = Partial<
  Pick<
    Note,
    "title" | "content" | "color" | "is_starred" | "is_deleted" | "folder_id"
  >
>;

export type NoteProtectionUpdateFields = NoteUpdateFields &
  Partial<
    Pick<
      Note,
      | "encrypted_content"
      | "encryption_alg"
      | "encryption_kdf"
      | "encryption_salt"
      | "encryption_nonce"
    >
  > & {
    protection_mode?: ProtectionMode;
    protection_password?: string;
  };

const emptySearchFilters: NoteSearchFilters = {
  title: "",
  tags: [],
  createdFrom: "",
  createdTo: "",
  updatedFrom: "",
  updatedTo: "",
};

function queryFor(state: NotesState): string {
  const params = new URLSearchParams();
  if (state.filter === "starred") params.set("starred", "true");
  if (state.filter === "trash") params.set("trash", "true");
  if (state.filter === "all" && state.folderID) {
    params.set("folder_id", state.folderID);
  }
  if (state.search.trim()) params.set("q", state.search.trim());
  if (state.searchFilters.title.trim()) {
    params.set("title", state.searchFilters.title.trim());
  }
  if (state.searchFilters.tags.length) {
    params.set("tags", state.searchFilters.tags.join(","));
  }
  if (state.searchFilters.createdFrom) {
    params.set("created_from", state.searchFilters.createdFrom);
  }
  if (state.searchFilters.createdTo) {
    params.set("created_to", state.searchFilters.createdTo);
  }
  if (state.searchFilters.updatedFrom) {
    params.set("updated_from", state.searchFilters.updatedFrom);
  }
  if (state.searchFilters.updatedTo) {
    params.set("updated_to", state.searchFilters.updatedTo);
  }
  const timeZone = get(preferencesStore).timeZone;
  if (timeZone) params.set("timezone", timeZone);
  return params.toString();
}

function createNotesStore() {
  let loadSequence = 0;
  const { subscribe, update } = writable<NotesState>({
    notes: [],
    selected: null,
    workspaceView: "notes",
    filter: "all",
    folderID: null,
    folderBrowserOpen: false,
    search: "",
    searchFilters: { ...emptySearchFilters },
    loading: false,
    error: "",
  });

  function applyUpdatedNote(updated: Note): void {
    update((current) => ({
      ...current,
      selected:
        current.selected?.id === updated.id
          ? {
              ...current.selected,
              ...updated,
              tags: updated.tags ?? current.selected.tags,
            }
          : current.selected,
      notes: current.notes.map((note) =>
        note.id === updated.id
          ? { ...note, ...updated, tags: updated.tags ?? note.tags }
          : note,
      ),
    }));
  }

  async function updateNote(
    noteId: string,
    fields: NoteProtectionUpdateFields,
  ): Promise<Note> {
    const updated = await apiFetch<Note>(`/notes/${noteId}`, {
      method: "PUT",
      body: JSON.stringify(fields),
    });
    applyUpdatedNote(updated);
    return updated;
  }

  async function load() {
    const sequence = ++loadSequence;
    update((state) => ({ ...state, loading: true, error: "" }));
    const state = get({ subscribe });
    const query = queryFor(state);

    try {
      const notes = await apiFetch<Note[]>(
        `/notes${query ? `?${query}` : ""}`,
      );
      if (sequence !== loadSequence) return;

      update((current) => ({
        ...current,
        notes,
        selected: current.selected
          ? notes.find((note) => note.id === current.selected?.id) ||
            current.selected
          : null,
        loading: false,
        error: "",
      }));
    } catch (error) {
      if (sequence !== loadSequence) return;

      update((current) => ({
        ...current,
        loading: false,
        error:
          error instanceof Error ? error.message : t("loadNotesFailed"),
      }));
    }
  }

  return {
    subscribe,
    load,
    async create(folderID?: string | null) {
      const state = get({ subscribe });
      const targetFolderID =
        folderID === undefined && state.filter === "all"
          ? state.folderID
          : (folderID ?? null);
      const note = await apiFetch<Note>("/notes", {
        method: "POST",
        body: JSON.stringify({
          title: t("untitled"),
          content: "",
          color: "",
          folder_id: targetFolderID,
        }),
      });
      update((state) => ({
        ...state,
        workspaceView: "notes",
        filter: "all",
        folderID: targetFolderID,
        folderBrowserOpen: false,
        search: "",
        searchFilters: { ...emptySearchFilters },
        selected: note,
      }));
      await load();
      update((state) => ({ ...state, selected: note }));
    },
    select(note: Note) {
      update((state) => ({ ...state, selected: note }));
    },
    clearSelection() {
      update((state) => ({ ...state, selected: null }));
    },
    async setFilter(filter: NoteFilter) {
      update((state) => ({
        ...state,
        workspaceView: "notes",
        filter,
        folderID: null,
        folderBrowserOpen: false,
        selected: null,
      }));
      await load();
    },
    async setFolder(folderID: string | null) {
      update((state) => ({
        ...state,
        workspaceView: "notes",
        filter: "all",
        folderID,
        folderBrowserOpen: false,
        selected: null,
      }));
      await load();
    },
    showFolderBrowser() {
      update((state) => ({
        ...state,
        workspaceView: "notes",
        filter: "all",
        folderBrowserOpen: true,
      }));
    },
    async setWorkspaceView(view: WorkspaceView) {
      update((state) => ({
        ...state,
        workspaceView: view,
        folderBrowserOpen: false,
      }));
      if (view === "index") await load();
    },
    async setSearch(search: string) {
      update((state) => ({ ...state, search, folderBrowserOpen: false }));
      await load();
    },
    async setSearchFilters(fields: Partial<NoteSearchFilters>) {
      update((state) => ({
        ...state,
        searchFilters: {
          ...state.searchFilters,
          ...fields,
          tags: fields.tags ?? state.searchFilters.tags,
        },
      }));
      await load();
    },
    async clearSearchFilters() {
      update((state) => ({
        ...state,
        search: "",
        searchFilters: { ...emptySearchFilters },
      }));
      await load();
    },
    async updateSelected(fields: NoteUpdateFields) {
      const state = get({ subscribe });
      if (!state.selected) return;

      const selectedID = state.selected.id;
      update((current) => ({ ...current, error: "" }));
      await updateNote(selectedID, fields);
    },
    async updateProtection(fields: NoteProtectionUpdateFields): Promise<Note | null> {
      const state = get({ subscribe });
      if (!state.selected) return null;

      update((current) => ({ ...current, error: "" }));
      return updateNote(state.selected.id, fields);
    },
    async verifyPassword(noteId: string, password: string): Promise<Note> {
      const response = await apiFetch<{ success: boolean; note: Note }>(
        `/notes/${noteId}/verify-password`,
        {
          method: "POST",
          body: JSON.stringify({ password }),
        },
      );
      applyUpdatedNote(response.note);
      return response.note;
    },
    async getByID(noteId: string): Promise<Note> {
      const state = get({ subscribe });
      if (state.selected?.id === noteId) return state.selected;
      const listed = state.notes.find((note) => note.id === noteId);
      if (listed) return listed;
      return apiFetch<Note>(`/notes/${noteId}`);
    },
    updateNote,
    async moveToFolder(noteIDs: string[], folderID: string | null) {
      const uniqueNoteIDs = [...new Set(noteIDs)];
      if (uniqueNoteIDs.length === 0) return;

      await apiFetch<{ updated_count: number }>("/notes/move", {
        method: "POST",
        body: JSON.stringify({
          note_ids: uniqueNoteIDs,
          folder_id: folderID,
        }),
      });

      update((state) => ({
        ...state,
        selected:
          state.selected && uniqueNoteIDs.includes(state.selected.id)
            ? { ...state.selected, folder_id: folderID }
            : state.selected,
        notes: state.notes.map((note) =>
          uniqueNoteIDs.includes(note.id)
            ? { ...note, folder_id: folderID, updated_at: new Date().toISOString() }
            : note,
        ),
      }));
    },
    replaceSelected(note: Note) {
      update((state) => ({
        ...state,
        selected: note,
        notes: state.notes.map((item) => (item.id === note.id ? note : item)),
      }));
    },
    async deletePermanent(noteId: string) {
      await apiFetch<void>(`/notes/${noteId}`, { method: "DELETE" });
      update((state) => ({
        ...state,
        selected: state.selected?.id === noteId ? null : state.selected,
        notes: state.notes.filter((note) => note.id !== noteId),
      }));
    },
    async emptyTrash() {
      const state = get({ subscribe });
      const trashNoteIds = new Set(
        state.notes.filter((note) => note.is_deleted).map((note) => note.id),
      );

      await apiFetch<{ deleted_count: number }>("/notes/trash", {
        method: "DELETE",
      });

      update((current) => ({
        ...current,
        selected:
          current.selected &&
          (current.selected.is_deleted || trashNoteIds.has(current.selected.id))
            ? null
            : current.selected,
        notes:
          current.filter === "trash"
            ? []
            : current.notes.filter((note) => !note.is_deleted),
      }));
    },
  };
}

export const notesStore = createNotesStore();
