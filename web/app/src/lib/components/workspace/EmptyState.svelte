<script lang="ts">
  import type { NoteFilter } from "../../stores/notes";
  import { preferencesStore, t, type MessageKey } from "../../stores/preferences";

  export let filter: NoteFilter = "all";

  let titleKey: MessageKey = "emptyNoteList";
  let subtitleKey: MessageKey = "emptyNoteListSubtitle";

  $: titleKey =
    filter === "trash"
      ? "emptyTrashTitle"
      : filter === "starred"
        ? "emptyStarredTitle"
        : "emptyNoteList";
  $: subtitleKey =
    filter === "trash"
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
