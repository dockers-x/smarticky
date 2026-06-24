<script lang="ts">
  import { Star, Trash2 } from "@lucide/svelte";
  import type { Note } from "../../api/types";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";

  export let note: Note;
  export let active = false;
  export let selected = false;
  export let dragNoteIDs: string[] = [];
  export let onToggleSelected: (noteID: string) => void = () => {};
  export let onToggleStar: (note: Note) => void | Promise<void> = () => {};
  export let onDelete: (note: Note) => void | Promise<void> = () => {};

  function plainTextPreview(content: string): string {
    return content
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
  }

  $: preview =
    plainTextPreview(note.content ?? "").slice(0, 96) ||
    t("contentEmpty", $preferencesStore.language);
  $: noteTitle = note.title || t("untitled", $preferencesStore.language);
  $: visibleTags = note.tags?.slice(0, 3) ?? [];
  $: hiddenTagCount = Math.max((note.tags?.length ?? 0) - visibleTags.length, 0);
  $: noteDate = new Date(note.updated_at).toLocaleString(
    $preferencesStore.language === "zh" ? "zh-CN" : "en-US",
    {
      month: "numeric",
      day: "numeric",
      year: "numeric",
      hour: "numeric",
      minute: "2-digit",
      timeZone: $preferencesStore.timeZone,
    },
  );

  function startDrag(event: DragEvent): void {
    const ids = selected && dragNoteIDs.length > 0 ? dragNoteIDs : [note.id];
    event.dataTransfer?.setData(
      "application/x-smarticky-note-ids",
      JSON.stringify(ids),
    );
    event.dataTransfer?.setData("text/plain", noteTitle);
    if (event.dataTransfer) event.dataTransfer.effectAllowed = "move";
  }
</script>

<article
  class:active
  class:selected
  class="note-card"
  draggable={!note.is_deleted}
  on:dragstart={startDrag}
>
  <div class="note-card__top">
    <label class="note-card__check" aria-label={t("selectedNotes", $preferencesStore.language)}>
      <input
        type="checkbox"
        checked={selected}
        on:change={() => onToggleSelected(note.id)}
      />
    </label>
    <button
      class="note-card__body"
      type="button"
      aria-pressed={active}
      on:click={() => notesStore.select(note)}
    >
      <span class="note-card__title">{noteTitle}</span>
      <span class="note-card__preview">{preview}</span>
    </button>
    <div class="note-card__actions">
      <button
        type="button"
        aria-label={note.is_starred
          ? t("starRemoved", $preferencesStore.language)
          : t("star", $preferencesStore.language)}
        aria-pressed={note.is_starred}
        title={note.is_starred
          ? t("starRemoved", $preferencesStore.language)
          : t("star", $preferencesStore.language)}
        on:click={() => void onToggleStar(note)}
      >
        <Star
          size={15}
          strokeWidth={2}
          fill={note.is_starred ? "currentColor" : "none"}
          aria-hidden="true"
        />
      </button>
      <button
        type="button"
        aria-label={note.is_deleted
          ? t("deleteForever", $preferencesStore.language)
          : t("trashNote", $preferencesStore.language)}
        title={note.is_deleted
          ? t("deleteForever", $preferencesStore.language)
          : t("trashNote", $preferencesStore.language)}
        on:click={() => void onDelete(note)}
      >
        <Trash2 size={15} strokeWidth={2} aria-hidden="true" />
      </button>
    </div>
  </div>
  <span class="note-card__meta">
    {#if visibleTags.length}
      <span class="note-card__tags" aria-label={t("tags", $preferencesStore.language)}>
        {#each visibleTags as tag (tag.id)}
          <span class="note-card__tag">{tag.name}</span>
        {/each}
        {#if hiddenTagCount > 0}
          <span class="note-card__tag">+{hiddenTagCount}</span>
        {/if}
      </span>
    {/if}
    <time datetime={note.updated_at}>
      {noteDate}
    </time>
  </span>
</article>
