const invalidFilenameCharacters = /[<>:"/\\|?*\u0000-\u001F]/g;
const maxFilenameBaseLength = 120;

export function markdownDownloadFilename(title: string, fallbackTitle = "Untitled"): string {
  const rawTitle = title.trim() || fallbackTitle.trim() || "Untitled";
  const base =
    rawTitle
      .replace(invalidFilenameCharacters, "-")
      .replace(/\s+/g, " ")
      .replace(/[. ]+$/g, "")
      .slice(0, maxFilenameBaseLength)
      .trim() || "Untitled";

  return /\.md$/i.test(base) ? base : `${base}.md`;
}

export function downloadMarkdownFile(
  title: string,
  content: string,
  fallbackTitle = "Untitled",
): void {
  const blob = new Blob([content], { type: "text/markdown;charset=utf-8" });
  const href = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = href;
  link.download = markdownDownloadFilename(title, fallbackTitle);
  link.rel = "noopener";
  document.body.append(link);
  link.click();
  link.remove();
  window.setTimeout(() => URL.revokeObjectURL(href), 0);
}
