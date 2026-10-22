import { describe, expect, it } from "vitest";
import { notePreview } from "./notePreview";

const labels = { empty: "No content", locked: "Note locked" };

describe("note previews", () => {
  it("preserves readable content without Markdown decoration", () => {
    expect(notePreview({ content: "# A useful **preview**\n> with `code`", content_redacted: false }, labels))
      .toBe("A useful preview with code");
  });

  it("shows labels instead of destinations for links, references and images", () => {
    expect(notePreview({
      content: "[Link](https://example.test) [[note-id|Page name]] ![Diagram](image.png)",
      content_redacted: false,
    }, labels)).toBe("Link Page name Diagram");
  });

  it("does not expose redacted content, even if a previous body remains in memory", () => {
    expect(notePreview({ content: "Private content", content_redacted: true }, labels)).toBe("Note locked");
  });

  it("does not mislabel a locked page as an empty note", () => {
    expect(notePreview({ content: "", content_redacted: true }, labels)).toBe("Note locked");
    expect(notePreview({ content: "", content_redacted: false }, labels)).toBe("No content");
  });

  it("keeps long previews bounded without changing the saved body", () => {
    const note = { content: "Readable content ".repeat(40), content_redacted: false };
    expect(notePreview(note, labels)).toHaveLength(96);
    expect(note.content).toBe("Readable content ".repeat(40));
  });

  it("keeps CJK text readable and omits code-only content", () => {
    expect(notePreview({ content: "## 纸张上的想法\n\n- 平静地书写", content_redacted: false }, labels))
      .toBe("纸张上的想法 平静地书写");
    expect(notePreview({ content: "```ts\nconst secret = true;\n```", content_redacted: false }, labels))
      .toBe("No content");
  });
});
