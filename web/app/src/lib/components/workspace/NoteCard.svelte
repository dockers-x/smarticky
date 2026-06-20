<script lang="ts">
  import type { Note } from "../../api/types";
  import { notesStore } from "../../stores/notes";

  export let note: Note;
  export let active = false;

  $: preview = note.content
    ? note.content.replace(/\s+/g, " ").slice(0, 86)
    : "没有正文";
</script>

<button
  class:active
  class="note-card"
  type="button"
  aria-pressed={active}
  on:click={() => notesStore.select(note)}
>
  <span class="note-card__title">{note.title || "未命名"}</span>
  <span class="note-card__preview">{preview}</span>
  <span class="note-card__meta">
    <time datetime={note.updated_at}>
      {new Date(note.updated_at).toLocaleString()}
    </time>
  </span>
</button>
