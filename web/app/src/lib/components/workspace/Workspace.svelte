<script lang="ts">
  import { onMount } from "svelte";
  import { notesStore } from "../../stores/notes";
  import NoteList from "./NoteList.svelte";
  import Sidebar from "./Sidebar.svelte";

  onMount(() => {
    notesStore.load();
  });
</script>

<div class="workspace">
  <Sidebar />
  <NoteList />
  <section class="editor-pane" aria-label="编辑器">
    {#if $notesStore.selected}
      <p class="editor-empty-text">{$notesStore.selected.title}</p>
    {:else}
      <p class="editor-empty-text">选择一篇笔记，或新建一篇</p>
    {/if}
  </section>
</div>
