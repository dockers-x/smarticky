import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createEditorDraft } from "../components/editor/draftSave";
import { captureEditorMarkdown, type MarkdownEditorHandle } from "./markdown";

const initial = { title: "A quiet page", content: "First line" };

function mockEditor(noteID: string, readMarkdown: () => string) {
  return {
    noteID,
    getMarkdown: vi.fn(readMarkdown),
    insertMarkdown: vi.fn(),
    focus: vi.fn(),
  } satisfies MarkdownEditorHandle;
}

describe("live editor Markdown capture", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.clearAllTimers();
    vi.useRealTimers();
  });

  it("flushes the live snapshot before the 200ms Markdown listener fires", async () => {
    const persist = vi.fn().mockResolvedValue(undefined);
    const draft = createEditorDraft(initial, persist, vi.fn());
    let liveMarkdown = initial.content;
    const editor = mockEditor("note-a", () => liveMarkdown);
    const delayedChange = vi.fn((content: string) => draft.update({ content }));

    liveMarkdown = "First line with final characters";
    setTimeout(() => delayedChange(liveMarkdown), 200);
    vi.advanceTimersByTime(199);
    expect(draft.dirty).toBe(false);
    expect(delayedChange).not.toHaveBeenCalled();

    const snapshot = captureEditorMarkdown(editor, "note-a");
    expect(snapshot).toBe("First line with final characters");
    if (snapshot !== null) draft.update({ content: snapshot });

    expect(await draft.flush()).toBe(true);
    expect(persist).toHaveBeenCalledExactlyOnceWith({ content: "First line with final characters" });
    expect(delayedChange).not.toHaveBeenCalled();
    expect(draft.dirty).toBe(false);

    vi.advanceTimersByTime(1);
    expect(delayedChange).toHaveBeenCalledExactlyOnceWith("First line with final characters");
    expect(await draft.flush()).toBe(true);
    expect(persist).toHaveBeenCalledTimes(1);
  });

  it("does not apply a delayed snapshot owned by the previous note to the current draft", async () => {
    const persist = vi.fn().mockResolvedValue(undefined);
    const draft = createEditorDraft({ title: "Note B", content: "B saved content" }, persist, vi.fn());
    const previousEditor = mockEditor("note-a", () => "A final edit");

    setTimeout(() => {
      const snapshot = captureEditorMarkdown(previousEditor, "note-b");
      expect(snapshot).toBeNull();
      if (snapshot !== null) draft.update({ content: snapshot });
    }, 200);
    vi.advanceTimersByTime(200);

    expect(previousEditor.getMarkdown).not.toHaveBeenCalled();
    expect(draft.value.content).toBe("B saved content");
    expect(draft.dirty).toBe(false);
    expect(await draft.flush()).toBe(true);
    expect(persist).not.toHaveBeenCalled();
  });

  it("preserves the draft when no live editor is available", async () => {
    const persist = vi.fn().mockResolvedValue(undefined);
    const draft = createEditorDraft(initial, persist, vi.fn());
    const snapshot = captureEditorMarkdown(null, "note-a");

    expect(snapshot).toBeNull();
    if (snapshot !== null) draft.update({ content: snapshot });

    expect(draft.value.content).toBe("First line");
    expect(await draft.flush()).toBe(true);
    expect(persist).not.toHaveBeenCalled();
  });

  it("does not save an unchanged live snapshot", async () => {
    const persist = vi.fn().mockResolvedValue(undefined);
    const draft = createEditorDraft(initial, persist, vi.fn());
    const editor = mockEditor("note-a", () => "First line");
    const snapshot = captureEditorMarkdown(editor, "note-a");

    expect(snapshot).toBe("First line");
    if (snapshot !== null) draft.update({ content: snapshot });

    expect(draft.dirty).toBe(false);
    expect(await draft.flush()).toBe(true);
    expect(draft.status).toBe("saved");
    expect(persist).not.toHaveBeenCalled();
  });

  it("captures an intentionally emptied document instead of treating it as unavailable", async () => {
    const persist = vi.fn().mockResolvedValue(undefined);
    const draft = createEditorDraft(initial, persist, vi.fn());
    const editor = mockEditor("note-a", () => "");
    const snapshot = captureEditorMarkdown(editor, "note-a");

    expect(snapshot).toBe("");
    if (snapshot !== null) draft.update({ content: snapshot });

    expect(draft.dirty).toBe(true);
    expect(await draft.flush()).toBe(true);
    expect(persist).toHaveBeenCalledExactlyOnceWith({ content: "" });
    expect(draft.value.content).toBe("");
  });
});
