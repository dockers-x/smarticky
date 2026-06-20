<script lang="ts">
  import ToolsPanel from "../settings/ToolsPanel.svelte";
  import { authStore } from "../../stores/auth";
  import { notesStore, type NoteFilter } from "../../stores/notes";

  let toolsOpen = false;

  const filters: { id: NoteFilter; label: string }[] = [
    { id: "all", label: "全部笔记" },
    { id: "starred", label: "收藏" },
    { id: "trash", label: "废纸篓" },
  ];
</script>

<aside class="sidebar" aria-label="导航">
  <div class="sidebar__brand">Smarticky</div>
  <nav class="sidebar__nav">
    {#each filters as filter}
      <button
        class:active={$notesStore.filter === filter.id}
        type="button"
        aria-pressed={$notesStore.filter === filter.id}
        on:click={() => notesStore.setFilter(filter.id)}
      >
        {filter.label}
      </button>
    {/each}
  </nav>
  <button
    class="sidebar__tool"
    type="button"
    aria-expanded={toolsOpen}
    on:click={() => (toolsOpen = !toolsOpen)}
  >
    工具
  </button>
  {#if toolsOpen}
    <ToolsPanel user={$authStore.user} onClose={() => (toolsOpen = false)} />
  {/if}
</aside>
