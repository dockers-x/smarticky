import { get } from "svelte/store";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Note } from "../api/types";
import {
  createNotesStore,
  resolveNewNoteContext,
  resolveNewNoteFolderID,
} from "./notes";

const { apiFetchMock } = vi.hoisted(() => ({ apiFetchMock: vi.fn() }));

vi.mock("../api/client", () => ({ apiFetch: apiFetchMock }));

function note(id: string): Note {
  return {
    id,
    title: "Untitled",
    content: "",
    color: "",
    protection_mode: "none",
    content_redacted: false,
    is_starred: false,
    is_deleted: false,
    folder_id: null,
    tags: [],
    created_at: "2026-08-11T00:00:00Z",
    updated_at: "2026-08-11T00:00:00Z",
  };
}

describe("resolveNewNoteFolderID", () => {
  it("uses the active folder in the all-notes scope", () => {
    expect(resolveNewNoteFolderID("all", "folder-1")).toBe("folder-1");
  });

  it("creates unfiled notes outside a folder scope", () => {
    expect(resolveNewNoteFolderID("starred", null)).toBeNull();
    expect(resolveNewNoteFolderID("trash", null)).toBeNull();
  });

  it("translates the unfiled filter sentinel to an empty destination", () => {
    expect(resolveNewNoteFolderID("all", "unfiled")).toBeNull();
    expect(resolveNewNoteFolderID("all", null, "unfiled")).toBeNull();
  });

  it("keeps the unfiled view while sending an empty API destination", () => {
    expect(resolveNewNoteContext("all", "unfiled")).toEqual({
      requestFolderID: null,
      viewFolderID: "unfiled",
    });
  });

  it("honors an explicit destination", () => {
    expect(resolveNewNoteFolderID("starred", null, "folder-2")).toBe(
      "folder-2",
    );
    expect(resolveNewNoteFolderID("all", "folder-1", null)).toBeNull();
  });
});

describe("notesStore.create", () => {
  beforeEach(() => {
    apiFetchMock.mockReset();
  });

  it("does not restore the created note after later navigation", async () => {
    const created = note("new-note");
    let finishInitialLoads: (notes: Note[]) => void = () => {};
    const initialLoads = new Promise<Note[]>((resolve) => {
      finishInitialLoads = resolve;
    });

    apiFetchMock.mockImplementation(
      (path: string, init: RequestInit | undefined) => {
        if (init?.method === "POST") return Promise.resolve(created);
        if (path.includes("starred=true")) return Promise.resolve([]);
        return initialLoads;
      },
    );

    const store = createNotesStore();
    const createPromise = store.create();
    await vi.waitFor(() => expect(apiFetchMock).toHaveBeenCalledTimes(3));

    await store.setFilter("starred");
    finishInitialLoads([]);
    await createPromise;

    expect(get(store).filter).toBe("starred");
    expect(get(store).selected).toBeNull();
  });
});
