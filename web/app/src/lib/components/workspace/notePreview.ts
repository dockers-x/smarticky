import type { Note } from "../../api/types";

export function notePreview(
  note: Pick<Note, "content" | "content_redacted">,
  labels: { empty: string; locked: string },
): string {
  if (note.content_redacted) return labels.locked;

  const plainText = (note.content ?? "")
    .replace(/```[\s\S]*?```/g, " ")
    .replace(/`([^`]+)`/g, "$1")
    .replace(/\\([[\]])/g, "$1")
    .replace(/!\[([^\]]*)\]\([^)]+\)/g, "$1")
    .replace(/\[([^\]]+)\]\([^)]+\)/g, "$1")
    .replace(/\[\[([^\]|]+)\|([^\]]+)\]\]/g, "$2")
    .replace(/\[\[([^\]]+)\]\]/g, "$1")
    .replace(/^\s*\|?[\s:-]+\|[\s|:-]*$/gm, " ")
    .replace(/^\s{0,3}#{1,6}\s+/gm, "")
    .replace(/^\s{0,3}>\s?/gm, "")
    .replace(/^\s*[-*+]\s+/gm, "")
    .replace(/^\s*\d+\.\s+/gm, "")
    .replace(/[*_~>#|]/g, "")
    .replace(/\s+/g, " ")
    .trim();

  return plainText.slice(0, 96) || labels.empty;
}
