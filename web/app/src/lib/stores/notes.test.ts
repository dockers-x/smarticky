import { describe, expect, it } from "vitest";
import { resolveNewNoteContext, resolveNewNoteFolderID } from "./notes";

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
