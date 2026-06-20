<script lang="ts">
  import type { Note } from "../../api/types";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";

  export let note: Note;
  export let active = false;

  $: preview = note.content
    ? note.content.replace(/\s+/g, " ").slice(0, 86)
    : t("contentEmpty", $preferencesStore.language);
  $: noteTitle = note.title || t("untitled", $preferencesStore.language);
  $: noteDate = new Date(note.updated_at).toLocaleString(
    $preferencesStore.language === "zh" ? "zh-CN" : "en-US",
    {
      month: "numeric",
      day: "numeric",
      year: "numeric",
      hour: "numeric",
      minute: "2-digit",
    },
  );
</script>

<button
  class:active
  class="note-card"
  type="button"
  aria-pressed={active}
  on:click={() => notesStore.select(note)}
>
  <span class="note-card__title">{noteTitle}</span>
  <span class="note-card__preview">{preview}</span>
  <span class="note-card__meta">
    <time datetime={note.updated_at}>
      {noteDate}
    </time>
  </span>
</button>
