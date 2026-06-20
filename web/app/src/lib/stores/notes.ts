import { get, writable } from "svelte/store";
import { apiFetch } from "../api/client";
import type { Note } from "../api/types";
import { t } from "./preferences";

export type NoteFilter = "all" | "starred" | "trash";

interface NotesState {
  notes: Note[];
  selected: Note | null;
  filter: NoteFilter;
  search: string;
  loading: boolean;
  error: string;
}

function queryFor(state: NotesState): string {
  const params = new URLSearchParams();
  if (state.filter === "starred") params.set("starred", "true");
  if (state.filter === "trash") params.set("trash", "true");
  if (state.search.trim()) params.set("q", state.search.trim());
  return params.toString();
}

function createNotesStore() {
  let loadSequence = 0;
  const { subscribe, update } = writable<NotesState>({
    notes: [],
    selected: null,
    filter: "all",
    search: "",
    loading: false,
    error: "",
  });

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
    async create() {
      const note = await apiFetch<Note>("/notes", {
        method: "POST",
        body: JSON.stringify({ title: t("untitled"), content: "", color: "" }),
      });
      update((state) => ({
        ...state,
        filter: "all",
        search: "",
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
      update((state) => ({ ...state, filter, selected: null }));
      await load();
    },
    async setSearch(search: string) {
      update((state) => ({ ...state, search }));
      await load();
    },
    async updateSelected(
      fields: Partial<
        Pick<Note, "title" | "content" | "color" | "is_starred" | "is_deleted">
      >,
    ) {
      const state = get({ subscribe });
      if (!state.selected) return;

      const selectedID = state.selected.id;
      update((current) => ({ ...current, error: "" }));
      const updated = await apiFetch<Note>(`/notes/${selectedID}`, {
        method: "PUT",
        body: JSON.stringify(fields),
      });

      update((current) => ({
        ...current,
        selected: current.selected?.id === updated.id ? updated : current.selected,
        notes: current.notes.map((note) =>
          note.id === updated.id ? { ...note, ...updated } : note,
        ),
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
    replaceSelected(note: Note) {
      update((state) => ({
        ...state,
        selected: note,
        notes: state.notes.map((item) => (item.id === note.id ? note : item)),
      }));
    },
  };
}

export const notesStore = createNotesStore();
