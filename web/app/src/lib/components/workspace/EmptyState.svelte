<script lang="ts">
  import type { NoteFilter } from "../../stores/notes";
  import { preferencesStore, t, type MessageKey } from "../../stores/preferences";

  export let filter: NoteFilter = "all";
  export let folderActive = false;

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
  <div class="empty-state__mark" aria-hidden="true">✎</div>
  <h2>{t(titleKey, $preferencesStore.language)}</h2>
  <p>{t(subtitleKey, $preferencesStore.language)}</p>
</div>
