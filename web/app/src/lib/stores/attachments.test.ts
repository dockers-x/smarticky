import { get } from "svelte/store";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Attachment } from "../api/types";
import { attachmentsStore } from "./attachments";

const { apiFetchMock } = vi.hoisted(() => ({ apiFetchMock: vi.fn() }));

vi.mock("../api/client", () => ({ apiFetch: apiFetchMock }));

function attachment(id: number): Attachment {
  return {
    id,
    filename: `attachment-${id}.txt`,
    file_size: 12,
    created_at: "2026-10-23T00:00:00Z",
  };
}

function deferred<Value>() {
  let resolve!: (value: Value) => void;
  const promise = new Promise<Value>((resolvePromise) => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}

describe("attachmentsStore", () => {
  beforeEach(() => {
    apiFetchMock.mockReset();
  });

  it("keeps a newly requested note active when an earlier upload completes", async () => {
    const previousAttachment = attachment(1);
    const uploadedAttachment = attachment(2);
    const currentAttachment = attachment(3);
    const uploadRequest = deferred<Attachment>();
    const currentLoad = deferred<Attachment[]>();
    apiFetchMock
      .mockResolvedValueOnce([previousAttachment])
      .mockReturnValueOnce(uploadRequest.promise)
      .mockReturnValueOnce(currentLoad.promise)
      .mockResolvedValue([previousAttachment, uploadedAttachment]);

    await attachmentsStore.load("note-a");
    const uploading = attachmentsStore.upload(
      "note-a",
      new File(["uploaded"], "upload.txt"),
    );
    const loadingCurrent = attachmentsStore.load("note-b");

    uploadRequest.resolve(uploadedAttachment);
    await expect(uploading).resolves.toEqual(uploadedAttachment);
    currentLoad.resolve([currentAttachment]);
    await loadingCurrent;

    expect(get(attachmentsStore)).toEqual([currentAttachment]);
    expect(apiFetchMock).toHaveBeenCalledTimes(3);
  });

  it("does not replace the displayed note after an earlier upload completes", async () => {
    const uploadedAttachment = attachment(1);
    const currentAttachment = attachment(2);
    const uploadRequest = deferred<Attachment>();
    apiFetchMock
      .mockResolvedValueOnce([])
      .mockReturnValueOnce(uploadRequest.promise)
      .mockResolvedValueOnce([currentAttachment])
      .mockResolvedValue([uploadedAttachment]);

    await attachmentsStore.load("note-a");
    const uploading = attachmentsStore.upload(
      "note-a",
      new File(["uploaded"], "upload.txt"),
    );
    await attachmentsStore.load("note-b");
    uploadRequest.resolve(uploadedAttachment);
    await expect(uploading).resolves.toEqual(uploadedAttachment);

    expect(get(attachmentsStore)).toEqual([currentAttachment]);
    expect(apiFetchMock).toHaveBeenCalledTimes(3);
  });

  it("refreshes attachments after uploading to the current note", async () => {
    const existingAttachment = attachment(1);
    const uploadedAttachment = attachment(2);
    const uploadRequest = deferred<Attachment>();
    const refreshRequest = deferred<Attachment[]>();
    apiFetchMock
      .mockResolvedValueOnce([existingAttachment])
      .mockReturnValueOnce(uploadRequest.promise)
      .mockReturnValueOnce(refreshRequest.promise);

    await attachmentsStore.load("note-a");
    const uploading = attachmentsStore.upload(
      "note-a",
      new File(["uploaded"], "upload.txt"),
    );
    uploadRequest.resolve(uploadedAttachment);
    await vi.waitFor(() => expect(apiFetchMock).toHaveBeenCalledTimes(3));
    expect(get(attachmentsStore)).toEqual([existingAttachment]);

    refreshRequest.resolve([existingAttachment, uploadedAttachment]);
    await expect(uploading).resolves.toEqual(uploadedAttachment);
    expect(get(attachmentsStore)).toEqual([
      existingAttachment,
      uploadedAttachment,
    ]);
  });

  it("ignores an upload refresh that finishes after another note loads", async () => {
    const uploadedAttachment = attachment(1);
    const currentAttachment = attachment(2);
    const refreshRequest = deferred<Attachment[]>();
    apiFetchMock
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce(uploadedAttachment)
      .mockReturnValueOnce(refreshRequest.promise)
      .mockResolvedValueOnce([currentAttachment]);

    await attachmentsStore.load("note-a");
    const uploading = attachmentsStore.upload(
      "note-a",
      new File(["uploaded"], "upload.txt"),
    );
    await vi.waitFor(() => expect(apiFetchMock).toHaveBeenCalledTimes(3));
    await attachmentsStore.load("note-b");
    refreshRequest.resolve([uploadedAttachment]);
    await expect(uploading).resolves.toEqual(uploadedAttachment);

    expect(get(attachmentsStore)).toEqual([currentAttachment]);
  });

  it("returns an uploaded attachment even when its list refresh fails", async () => {
    const existingAttachment = attachment(1);
    const uploadedAttachment = attachment(2);
    apiFetchMock
      .mockResolvedValueOnce([existingAttachment])
      .mockResolvedValueOnce(uploadedAttachment)
      .mockRejectedValueOnce(new Error("Refresh failed"));

    await attachmentsStore.load("note-a");
    await expect(
      attachmentsStore.upload("note-a", new File(["uploaded"], "upload.txt")),
    ).resolves.toEqual(uploadedAttachment);

    expect(get(attachmentsStore)).toEqual([existingAttachment]);
  });

  it("still rejects a failed upload without refreshing attachments", async () => {
    const uploadError = new Error("Upload failed");
    apiFetchMock
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(uploadError);

    await attachmentsStore.load("note-a");
    await expect(
      attachmentsStore.upload("note-a", new File(["uploaded"], "upload.txt")),
    ).rejects.toBe(uploadError);

    expect(get(attachmentsStore)).toEqual([]);
    expect(apiFetchMock).toHaveBeenCalledTimes(2);
  });

  it("still rejects an explicitly requested load failure", async () => {
    const existingAttachment = attachment(1);
    const loadError = new Error("Load failed");
    apiFetchMock
      .mockResolvedValueOnce([existingAttachment])
      .mockRejectedValueOnce(loadError);

    await attachmentsStore.load("note-a");
    await expect(attachmentsStore.load("note-b")).rejects.toBe(loadError);

    expect(get(attachmentsStore)).toEqual([existingAttachment]);
  });
});
