<script lang="ts">
  import { FilePenLine, Plus, Search, X } from "@lucide/svelte";
  import type { NoteFilter } from "../../stores/notes";
  import { preferencesStore, t, type MessageKey } from "../../stores/preferences";

  export let filter: NoteFilter = "all";
  export let folderActive = false;
  export let showCreate = false;
  export let busy = false;
  export let onCreate: () => void = () => {};
  export let filtered = false;
  export let onClearFilters: () => void = () => {};

  let titleKey: MessageKey = "emptyNoteList";
  let subtitleKey: MessageKey = "emptyNoteListSubtitle";

  $: titleKey =
    folderActive
      ? "folderEmptyTitle"
      : filter === "trash"
      ? "emptyTrashTitle"
      : filter === "starred"
        ? "emptyStarredTitle"
        : "emptyNoteList";
  $: subtitleKey =
    folderActive
      ? "folderEmptySubtitle"
      : filter === "trash"
      ? "emptyTrashSubtitle"
      : filter === "starred"
        ? "emptyStarredSubtitle"
        : "emptyNoteListSubtitle";
</script>

<div class="empty-state">
  <div class="empty-state__mark" aria-hidden="true">
    {#if filtered}
      <Search size={27} strokeWidth={1.6} />
    {:else}
      <FilePenLine size={27} strokeWidth={1.6} />
    {/if}
  </div>
  <h2>{filtered
    ? $preferencesStore.language === "zh" ? "没有匹配的笔记" : "No matching notes"
    : t(titleKey, $preferencesStore.language)}</h2>
  <p>{filtered
    ? $preferencesStore.language === "zh"
      ? "试试其他关键词，或清除筛选查看此处的全部笔记。"
      : "Try another search, or clear the filters to see all notes in this view."
    : t(subtitleKey, $preferencesStore.language)}</p>
  {#if filtered}
    <button type="button" on:click={onClearFilters}>
      <X size={16} strokeWidth={1.8} aria-hidden="true" />
      <span>{t("clearSearchFilters", $preferencesStore.language)}</span>
    </button>
  {:else if showCreate}
    <button type="button" aria-busy={busy} disabled={busy} on:click={onCreate}>
      <Plus size={16} strokeWidth={2} aria-hidden="true" />
      <span>{t("newNote", $preferencesStore.language)}</span>
    </button>
  {/if}
</div>

<style>
  .empty-state {
    height: auto;
    min-height: 240px;
    padding: 28px 16px;
  }

  .empty-state h2 {
    font-family: var(--font-reader);
    font-size: 21px;
    line-height: 1.35;
    text-wrap: balance;
  }

  .empty-state p {
    color: var(--color-text-sub);
    text-wrap: pretty;
  }

  .empty-state button {
    min-height: 44px;
    justify-content: center;
  }
</style>
