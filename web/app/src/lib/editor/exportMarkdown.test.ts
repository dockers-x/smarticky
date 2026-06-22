import { describe, expect, it } from "vitest";
import { markdownDownloadFilename } from "./exportMarkdown";

describe("markdown export helpers", () => {
  it("uses the note title as a markdown filename", () => {
    expect(markdownDownloadFilename("会议纪要")).toBe("会议纪要.md");
  });

  it("keeps an existing markdown extension", () => {
    expect(markdownDownloadFilename("README.md")).toBe("README.md");
  });

  it("replaces filename characters that common filesystems reject", () => {
    expect(markdownDownloadFilename('Plan: Q2 / "Draft"?')).toBe("Plan- Q2 - -Draft--.md");
  });

  it("falls back when the note title is blank", () => {
    expect(markdownDownloadFilename("  ", "未命名")).toBe("未命名.md");
  });
});
