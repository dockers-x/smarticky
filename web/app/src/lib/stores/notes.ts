import { get, writable } from "svelte/store";
import { apiFetch } from "../api/client";
import type { Note } from "../api/types";

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
  const { subscribe, update } = writable<NotesState>({
    notes: [],
    selected: null,
    filter: "all",
    search: "",
    loading: false,
    error: "",
  });

  async function load() {
    update((state) => ({ ...state, loading: true, error: "" }));
    const state = get({ subscribe });
    const query = queryFor(state);
    const notes = await apiFetch<Note[]>(`/notes${query ? `?${query}` : ""}`);

    update((current) => ({
      ...current,
      notes,
      selected: current.selected
        ? notes.find((note) => note.id === current.selected?.id) ||
          current.selected
        : null,
      loading: false,
    }));
  }

  return {
    subscribe,
    load,
    async create() {
      const note = await apiFetch<Note>("/notes", {
        method: "POST",
        body: JSON.stringify({ title: "未命名", content: "", color: "" }),
      });
      await load();
      update((state) => ({
        ...state,
        selected: note,
        filter: "all",
        search: "",
      }));
    },
    select(note: Note) {
      update((state) => ({ ...state, selected: note }));
    },
    async setFilter(filter: NoteFilter) {
      update((state) => ({ ...state, filter, selected: null }));
      await load();
    },
    async setSearch(search: string) {
      update((state) => ({ ...state, search }));
      await load();
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
