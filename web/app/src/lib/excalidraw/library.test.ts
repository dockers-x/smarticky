import { describe, expect, it } from "vitest";
import {
  excalidrawLibraryReturnURL,
  parseExcalidrawLibraryJSON,
  whiteboardIDFromLibraryCallback,
} from "./library";

describe("excalidraw library callback helpers", () => {
  it("keeps whiteboard id in the return URL query so Excalidraw can append hash params", () => {
    window.history.replaceState({}, "", "/");

    expect(
      excalidrawLibraryReturnURL("123e4567-e89b-12d3-a456-426614174000"),
    ).toBe(
      `${window.location.origin}/?whiteboard=123e4567-e89b-12d3-a456-426614174000`,
    );
  });

  it("reads the callback whiteboard id from query before hash", () => {
    window.history.replaceState(
      {},
      "",
      "/?whiteboard=query-id#whiteboard=hash-id&addLibrary=https%3A%2F%2Flibraries.excalidraw.com%2Fdemo.excalidrawlib",
    );

    expect(whiteboardIDFromLibraryCallback()).toBe("query-id");
  });

  it("treats invalid stored library json as an empty library", () => {
    expect(parseExcalidrawLibraryJSON("{bad")).toEqual([]);
    expect(parseExcalidrawLibraryJSON("{}")).toEqual([]);
  });
});
