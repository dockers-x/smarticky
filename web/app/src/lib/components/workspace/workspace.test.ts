import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushSync, mount, unmount, type Component } from "svelte";
import { get } from "svelte/store";
import type { Note } from "../../api/types";
import { notesStore } from "../../stores/notes";
import { preferencesStore } from "../../stores/preferences";
import EmptyState from "./EmptyState.svelte";
import MobileNav from "./MobileNav.svelte";
import NoteCard from "./NoteCard.svelte";

vi.mock("svelte", async () => vi.importActual<typeof import("svelte")>(
  new URL("../../../../node_modules/svelte/src/index-client.js", import.meta.url).pathname,
));
vi.mock("../../api/client", () => ({ apiFetch: vi.fn().mockResolvedValue([]) }));

const cleanups: Array<() => Promise<void>> = [];

function renderComponent<Props extends Record<string, unknown>>(
  component: Component<Props>,
  props: Props,
): HTMLDivElement {
  const element = document.createElement("div");
  document.body.append(element);
  const instance = mount(component, { target: element, props });
  flushSync();
  cleanups.push(async () => {
    await unmount(instance);
    element.remove();
  });
  return element;
}

const note: Note = {
  id: "note-1",
  title: "A notebook page",
  content: "# A useful **preview**",
  color: "",
  protection_mode: "none",
  content_redacted: false,
  is_starred: false,
  is_deleted: false,
  folder_id: null,
  tags: [],
  created_at: "2026-10-22T12:00:00Z",
  updated_at: "2026-10-22T12:00:00Z",
};

beforeEach(async () => {
  preferencesStore.setLanguage("en");
  preferencesStore.setTimeZone("UTC");
  await notesStore.setFilter("all");
});

afterEach(async () => {
  for (const cleanup of cleanups.splice(0).reverse()) await cleanup();
});

describe("workspace navigation and cards", () => {
  it("keeps five primary mobile destinations visible", () => {
    const element = renderComponent(MobileNav, {});
    const buttons = element.querySelectorAll(".mobile-nav__row > button");
    expect(buttons).toHaveLength(5);
    expect(Array.from(buttons, (button) => button.getAttribute("aria-label"))).toEqual([
      "All notes", "Starred", "Notebook groups", "Index", "More",
    ]);
    expect(buttons[4].getAttribute("aria-haspopup")).toBe("dialog");
  });

  it("opens More as a dismissible dialog and restores focus", async () => {
    const element = renderComponent(MobileNav, {});
    const opener = element.querySelector<HTMLButtonElement>('[aria-label="More"]')!;
    opener.focus();
    opener.click();
    flushSync();
    await Promise.resolve();
    expect(element.querySelector('[role="dialog"]')?.getAttribute("aria-modal")).toBe("true");
    expect(element.querySelector('[role="dialog"]')?.textContent).toContain("Tags");
    document.activeElement?.dispatchEvent(new KeyboardEvent("keydown", {
      key: "Escape", bubbles: true, cancelable: true,
    }));
    flushSync();
    await Promise.resolve();
    expect(element.querySelector('[role="dialog"]')).toBeNull();
    expect(document.activeElement).toBe(opener);
  });

  it("clears the previous editor selection when navigating to the index", () => {
    notesStore.select(note);
    const element = renderComponent(MobileNav, {});
    element.querySelector<HTMLButtonElement>('[aria-label="Index"]')?.click();
    flushSync();
    expect(get(notesStore).selected).toBeNull();
    expect(get(notesStore).workspaceView).toBe("index");
  });

  it("offers filter recovery instead of a misleading create-note action", () => {
    const onClearFilters = vi.fn();
    const element = renderComponent(EmptyState, { filtered: true, showCreate: true, onClearFilters });
    expect(element.querySelector("h2")?.textContent).toBe("No matching notes");
    const button = element.querySelector<HTMLButtonElement>("button")!;
    expect(button.textContent).toContain("Clear filters");
    expect(element.textContent).not.toContain("New note");
    button.click();
    expect(onClearFilters).toHaveBeenCalledOnce();
  });

  it("preserves the create action for a genuinely empty notebook", () => {
    const onCreate = vi.fn();
    const element = renderComponent(EmptyState, { showCreate: true, onCreate });
    const button = element.querySelector<HTMLButtonElement>("button")!;
    expect(button.textContent).toContain("New note");
    button.click();
    expect(onCreate).toHaveBeenCalledOnce();
  });

  it("names card selection and identifies the currently opened page independently", () => {
    const element = renderComponent(NoteCard, { note, active: true });
    expect(element.querySelector("article")?.dataset.noteId).toBe("note-1");
    expect(element.querySelector('input[type="checkbox"]')?.getAttribute("aria-label"))
      .toBe("Select note: A notebook page");
    expect(element.querySelector(".note-card__body")?.getAttribute("aria-current")).toBe("true");
    expect(element.querySelector(".note-card__preview")?.textContent).toBe("A useful preview");
  });

  it("toggles bulk selection instead of unexpectedly opening the editor", () => {
    const onToggleSelected = vi.fn();
    const element = renderComponent(NoteCard, { note, selected: true, selectionMode: true, onToggleSelected });
    expect(element.querySelector(".note-card__body")?.getAttribute("aria-pressed")).toBe("true");
    expect(element.querySelector<HTMLInputElement>('input[type="checkbox"]')?.checked).toBe(true);
    element.querySelector<HTMLButtonElement>(".note-card__body")?.click();
    expect(onToggleSelected).toHaveBeenCalledWith("note-1");
    expect(get(notesStore).selected).toBeNull();
  });

  it("does not mislabel protected content or star actions", () => {
    const protectedNote = { ...note, content: "", content_redacted: true, is_starred: true };
    const element = renderComponent(NoteCard, { note: protectedNote });
    expect(element.querySelector(".note-card__preview")?.textContent).toBe("Note locked");
    expect(element.querySelector(".note-card__star")?.getAttribute("aria-label"))
      .toBe("Remove star: A notebook page");
  });

  it("prevents duplicate card actions without removing the focus return target", async () => {
    let finish: () => void = () => {};
    const onToggleStar = vi.fn(() => new Promise<void>((resolve) => { finish = resolve; }));
    const element = renderComponent(NoteCard, { note, onToggleStar });
    const button = element.querySelector<HTMLButtonElement>(".note-card__star")!;
    button.focus();
    button.click();
    flushSync();
    expect(button.getAttribute("aria-disabled")).toBe("true");
    expect(button.disabled).toBe(false);
    button.click();
    expect(onToggleStar).toHaveBeenCalledOnce();
    finish();
    await vi.waitFor(() => {
      flushSync();
      expect(button.getAttribute("aria-disabled")).toBe("false");
    });
    expect(document.activeElement).toBe(button);
  });
});
