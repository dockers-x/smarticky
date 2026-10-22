import { describe, expect, it, vi } from "vitest";
import { createEditorDraft, type EditorSaveStatus } from "./draftSave";

const initial = { title: "A quiet page", content: "First line" };

describe("editor draft saving", () => {
  it("reports pending changes before a debounced save starts", () => {
    const report = vi.fn();
    const draft = createEditorDraft(initial, vi.fn(), report);
    draft.update({ title: "A new title" });

    expect(draft.status).toBe("pending");
    expect(draft.dirty).toBe(true);
    expect(report).toHaveBeenCalledWith("pending");
  });

  it("saves title and content together without rewriting unchanged fields", async () => {
    const persist = vi.fn().mockResolvedValue(undefined);
    const draft = createEditorDraft(initial, persist, vi.fn());
    draft.update({ title: "Revised", content: "Revised text" });

    expect(await draft.flush()).toBe(true);
    expect(persist).toHaveBeenCalledExactlyOnceWith({ title: "Revised", content: "Revised text" });
    expect(draft.dirty).toBe(false);
    expect(draft.status).toBe("saved");
    await draft.flush();
    expect(persist).toHaveBeenCalledTimes(1);
  });

  it("does not hide a new pending edit immediately after an unchanged flush", async () => {
    const draft = createEditorDraft(initial, vi.fn(), vi.fn());
    const unchanged = draft.flush();
    draft.update({ content: "Unsaved edit" });

    expect(await unchanged).toBe(true);
    expect(draft.status).toBe("pending");
    expect(draft.dirty).toBe(true);
  });

  it("keeps a failed draft available for retry instead of treating it as saved", async () => {
    const persist = vi.fn().mockRejectedValueOnce(new Error("Offline")).mockResolvedValue(undefined);
    const draft = createEditorDraft(initial, persist, vi.fn());
    draft.update({ content: "Still here" });

    expect(await draft.flush()).toBe(false);
    expect(draft.status).toBe("error");
    expect(draft.value.content).toBe("Still here");
    expect(draft.dirty).toBe(true);
    expect(await draft.flush()).toBe(true);
    expect(draft.dirty).toBe(false);
    expect(persist).toHaveBeenCalledTimes(2);
  });

  it("serializes writes and includes edits made while a request is in flight", async () => {
    let finishFirst: () => void = () => {};
    const firstRequest = new Promise<void>((resolve) => {
      finishFirst = resolve;
    });
    const persist = vi.fn().mockReturnValueOnce(firstRequest).mockResolvedValue(undefined);
    const states: EditorSaveStatus[] = [];
    const draft = createEditorDraft(initial, persist, (state) => states.push(state));
    draft.update({ content: "First edit" });
    const firstFlush = draft.flush();
    draft.update({ title: "Second edit", content: "Newest edit" });
    const secondFlush = draft.flush();

    expect(firstFlush).toBe(secondFlush);
    expect(persist).toHaveBeenCalledTimes(1);
    expect(draft.status).toBe("saving");
    finishFirst();

    expect(await firstFlush).toBe(true);
    expect(persist).toHaveBeenLastCalledWith({ title: "Second edit", content: "Newest edit" });
    expect(states).toEqual(["pending", "saving", "saving", "saved"]);
    expect(draft.dirty).toBe(false);
  });

  it("writes a reverted value if an older value is already being saved", async () => {
    let finishFirst: () => void = () => {};
    const persist = vi.fn().mockReturnValueOnce(new Promise<void>((resolve) => {
      finishFirst = resolve;
    })).mockResolvedValue(undefined);
    const draft = createEditorDraft(initial, persist, vi.fn());
    draft.update({ content: "Temporary" });
    const saving = draft.flush();
    draft.update({ content: initial.content });
    finishFirst();

    expect(await saving).toBe(true);
    expect(persist).toHaveBeenLastCalledWith({ content: initial.content });
  });

  it("accepts unlocked plaintext as the saved baseline without sending it back", () => {
    const persist = vi.fn();
    const draft = createEditorDraft({ ...initial, content: "" }, persist, vi.fn());
    draft.update({ title: "Pending title" });
    draft.accept({ content: "Decrypted text" });

    expect(draft.changes).toEqual({ title: "Pending title" });
    expect(draft.value.content).toBe("Decrypted text");
  });
});
