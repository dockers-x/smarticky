import type { Note } from "../../api/types";

export function notePreview(
  note: Pick<Note, "content" | "content_redacted">,
  labels: { empty: string; locked: string },
  search = "",
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

  const index = search ? findMatch(plainText, search)?.index ?? -1 : -1;
  const start = Math.max(0, index - 24);
  return (start ? "…" : "") + (plainText.slice(start, start + 96) || labels.empty);
}

function findMatch(text: string, search: string): RegExpExecArray | null {
  const literal = search.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  return new RegExp(literal, "iu").exec(text);
}

export function matchExcerpt(text: string, search: string) {
  const match = search && findMatch(text, search);
  if (!match) return { before: text, match: "", after: "" };
  return { before: text.slice(0, match.index), match: match[0], after: text.slice(match.index + match[0].length) };
}
