<script lang="ts">
  import { notesStore } from "../../stores/notes";
  import EmptyState from "./EmptyState.svelte";
  import NoteCard from "./NoteCard.svelte";
</script>

<section class="note-list-pane" aria-label="笔记列表">
  <div class="note-list-toolbar">
    <input
      type="search"
      aria-label="搜索笔记"
      placeholder="搜索笔记"
      value={$notesStore.search}
      on:input={(event) => notesStore.setSearch(event.currentTarget.value)}
    />
  </div>

  {#if $notesStore.error}
    <div class="note-list-message" role="alert">{$notesStore.error}</div>
  {:else if $notesStore.loading}
    <div class="note-list-message">正在加载笔记</div>
  {:else if $notesStore.notes.length === 0}
    <EmptyState />
  {:else}
    <div class="note-card-list">
      {#each $notesStore.notes as note (note.id)}
        <NoteCard {note} active={$notesStore.selected?.id === note.id} />
      {/each}
    </div>
  {/if}

  <button
    class="new-note-fab"
    type="button"
    aria-label="新建笔记"
    on:click={() => notesStore.create()}
  >
    +
  </button>
</section>
