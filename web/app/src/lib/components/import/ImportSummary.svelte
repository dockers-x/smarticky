<script lang="ts">
  import type { ImportPreview, ImportResult } from "../../api/imports";
  import { preferencesStore, t } from "../../stores/preferences";

  export let preview: ImportPreview | null = null;
  export let result: ImportResult | null = null;

  $: rows = preview
    ? [
        { label: t("notes", $preferencesStore.language), value: preview.note_count },
        { label: t("tags", $preferencesStore.language), value: preview.tag_count },
        { label: t("attachments", $preferencesStore.language), value: preview.resource_count },
        { label: t("warnings", $preferencesStore.language), value: preview.warning_count },
      ]
    : result
      ? [
          { label: t("imported", $preferencesStore.language), value: result.imported_count },
          { label: t("skipped", $preferencesStore.language), value: result.skipped_count },
          { label: t("failed", $preferencesStore.language), value: result.failed_count },
        ]
      : [];
</script>

<div class="import-summary" role="status">
  {#each rows as row}
    <div class="import-summary__item">
      <span>{row.value}</span>
      <small>{row.label}</small>
    </div>
  {/each}
</div>
