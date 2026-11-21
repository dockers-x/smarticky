<script lang="ts">
  import { Star, Trash2 } from "@lucide/svelte";
  import type { Note } from "../../api/types";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";
  import { noteDates } from "./noteDates";
  import { notePreview, matchExcerpt } from "./notePreview";

  export let note: Note;
  export let search = "";
  export let active = false;
  export let selected = false;
  export let selectionMode = false;
  export let dragNoteIDs: string[] = [];
  export let onToggleSelected: (noteID: string) => void = () => {};
  export let onToggleStar: (note: Note) => void | Promise<void> = () => {};
  export let onDelete: (note: Note) => void | Promise<void> = () => {};

  let pending = false;

  $: preview = notePreview(note, {
    empty: t("contentEmpty", $preferencesStore.language),
    locked: t("noteLocked", $preferencesStore.language),
  }, search);
  $: titleMatch = matchExcerpt(noteTitle, search);
  $: previewMatch = matchExcerpt(preview, note.content_redacted ? "" : search);
  $: noteTitle = note.title || t("untitled", $preferencesStore.language);
  $: selectionLabel = `${$preferencesStore.language === "zh" ? "选择笔记" : "Select note"}: ${noteTitle}`;
  $: starLabel = note.is_starred
    ? $preferencesStore.language === "zh" ? "取消收藏" : "Remove star"
    : t("star", $preferencesStore.language);
  $: visibleTags = note.tags?.slice(0, 2) ?? [];
  $: hiddenTagCount = Math.max((note.tags?.length ?? 0) - visibleTags.length, 0);
  $: noteDate = noteDates($preferencesStore.language, $preferencesStore.timeZone).short(new Date(note.updated_at));

  function startDrag(event: DragEvent): void {
    const ids = selected && dragNoteIDs.length > 0 ? dragNoteIDs : [note.id];
    event.dataTransfer?.setData(
      "application/x-smarticky-note-ids",
      JSON.stringify(ids),
    );
    event.dataTransfer?.setData("text/plain", noteTitle);
    if (event.dataTransfer) event.dataTransfer.effectAllowed = "move";
  }

  async function runAction(action: (target: Note) => void | Promise<void>): Promise<void> {
    if (pending) return;
    pending = true;
    try {
      await action(note);
    } finally {
      pending = false;
    }
  }
</script>

<article
  class:active
  class:selected
  class:selection-mode={selectionMode}
  class="note-card"
  data-note-id={note.id}
  aria-label={noteTitle}
  aria-busy={pending}
  draggable={!note.is_deleted && !pending}
  on:dragstart={startDrag}
>
  <div class="note-card__top">
    <button
      class="note-card__body"
      type="button"
      aria-label={selectionMode ? selectionLabel : noteTitle}
      aria-describedby={`note-preview-${note.id}`}
      aria-current={active ? "true" : undefined}
      aria-pressed={selectionMode ? selected : active}
      on:click={() => selectionMode ? onToggleSelected(note.id) : notesStore.select(note)}
    >
      <span class="note-card__title" title={noteTitle}>{titleMatch.before}{#if titleMatch.match}<mark>{titleMatch.match}</mark>{/if}{titleMatch.after}</span>
      <span class="note-card__preview" id={`note-preview-${note.id}`}>{previewMatch.before}{#if previewMatch.match}<mark>{previewMatch.match}</mark>{/if}{previewMatch.after}</span>
    </button>
    <label class="note-card__check" title={selectionLabel}>
      <input
        type="checkbox"
        aria-label={selectionLabel}
        checked={selected}
        on:change={() => onToggleSelected(note.id)}
      />
    </label>
  </div>
  <div class="note-card__meta">
    <time datetime={note.updated_at}>
      {noteDate}
    </time>
    {#if visibleTags.length}
      <span class="note-card__tags" aria-label={t("tags", $preferencesStore.language)}>
        {#each visibleTags as tag (tag.id)}
          <span class="note-card__tag" title={tag.name}>{tag.name}</span>
        {/each}
        {#if hiddenTagCount > 0}
          <span class="note-card__tag">+{hiddenTagCount}</span>
        {/if}
      </span>
    {/if}
    <div class="note-card__actions">
      <button
        class="note-card__star"
        type="button"
        aria-label={`${starLabel}: ${noteTitle}`}
        aria-pressed={note.is_starred}
        title={starLabel}
        aria-disabled={pending}
        on:click={() => void runAction(onToggleStar)}
      >
        <Star
          size={15}
          strokeWidth={2}
          fill={note.is_starred ? "currentColor" : "none"}
          aria-hidden="true"
        />
      </button>
      <button
        class="note-card__trash"
        type="button"
        aria-label={note.is_deleted
          ? t("deleteForever", $preferencesStore.language)
          : t("trashNote", $preferencesStore.language)}
        title={note.is_deleted
          ? t("deleteForever", $preferencesStore.language)
          : t("trashNote", $preferencesStore.language)}
        aria-disabled={pending}
        on:click={() => void runAction(onDelete)}
      >
        <Trash2 size={15} strokeWidth={2} aria-hidden="true" />
      </button>
    </div>
  </div>
</article>

<style>
  .note-card {
    padding: 12px 10px 8px;
  }

  .note-card__top {
    grid-template-columns: minmax(0, 1fr) 32px;
    gap: 8px;
  }

  .note-card__check {
    width: 32px;
    height: 32px;
    margin-top: -5px;
    cursor: pointer;
  }

  .note-card__check input {
    width: 16px;
    height: 16px;
    cursor: pointer;
  }

  .note-card__body {
    min-height: 44px;
    border-radius: 4px;
    cursor: pointer;
  }

  .note-card.selection-mode .note-card__body {
    cursor: default;
  }

  .note-card__title {
    padding-right: 0;
    font-size: 14px;
    line-height: 1.45;
    line-clamp: 2;
    -webkit-line-clamp: 2;
  }

  .note-card__preview {
    margin-top: 3px;
    line-height: 1.5;
  }

  .note-card__meta {
    min-width: 0;
    justify-content: flex-start;
    gap: 6px;
    padding-left: 0;
    font-size: 12px;
  }

  .note-card__meta time {
    flex: 0 0 auto;
    font-variant-numeric: tabular-nums;
    margin: 0;
    white-space: nowrap;
  }

  .note-card__tags {
    flex-wrap: nowrap;
    overflow: hidden;
  }

  .note-card__tag {
    max-width: 80px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .note-card__actions {
    margin-inline-start: auto;
    position: static;
    flex: 0 0 auto;
    gap: 2px;
  }

  .note-card__actions button {
    width: 32px;
    height: 32px;
    color: var(--color-text-sub);
  }

  .note-card__actions button[aria-disabled="true"] {
    opacity: 0.5;
    cursor: wait;
  }

  .note-card.selected:not(.active) {
    border-color: var(--color-divider-em);
    background: color-mix(in srgb, var(--color-ink-accent) 7%, var(--color-card));
    box-shadow: inset 3px 0 0 var(--color-ink-accent);
  }

  @media (min-width: 961px) and (pointer: fine) {
    .note-card {
      gap: 4px;
      padding: 10px 12px 6px;
    }

    .note-card__body {
      min-height: 36px;
    }

    .note-card__title {
      margin: 0;
      line-height: 1.35;
      line-clamp: 1;
      -webkit-line-clamp: 1;
    }

    .note-card__preview {
      margin-top: 2px;
      line-height: 1.4;
      line-clamp: 1;
      -webkit-line-clamp: 1;
    }
  }

  @media (max-width: 960px), (pointer: coarse) {
    .note-card__top {
      grid-template-columns: minmax(0, 1fr) 44px;
    }

    .note-card__check {
      width: 44px;
      height: 44px;
    }

    .note-card__meta {
      padding-left: 0;
    }

    .note-card__actions button {
      width: 44px;
      height: 44px;
    }
  }
</style>
