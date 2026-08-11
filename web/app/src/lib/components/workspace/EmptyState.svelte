<script lang="ts">
  import { FilePenLine, Plus } from "@lucide/svelte";
  import type { NoteFilter } from "../../stores/notes";
  import { preferencesStore, t, type MessageKey } from "../../stores/preferences";

  export let filter: NoteFilter = "all";
  export let folderActive = false;
  export let showCreate = false;
  export let busy = false;
  export let onCreate: () => void = () => {};

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
    <FilePenLine size={27} strokeWidth={1.6} />
  </div>
  <h2>{t(titleKey, $preferencesStore.language)}</h2>
  <p>{t(subtitleKey, $preferencesStore.language)}</p>
  {#if showCreate}
    <button type="button" aria-busy={busy} disabled={busy} on:click={onCreate}>
      <Plus size={16} strokeWidth={2} aria-hidden="true" />
      <span>{t("newNote", $preferencesStore.language)}</span>
    </button>
  {/if}
</div>
