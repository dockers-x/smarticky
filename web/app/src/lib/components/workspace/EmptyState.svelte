<script lang="ts">
  import { preferencesStore, t } from "../../stores/preferences";
</script>

<div class="empty-state">
  <div class="empty-state__mark" aria-hidden="true">✎</div>
  <h2>{t("emptyNoteList", $preferencesStore.language)}</h2>
  <p>{t("emptyNoteListSubtitle", $preferencesStore.language)}</p>
</div>
