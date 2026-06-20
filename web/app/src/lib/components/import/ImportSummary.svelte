<script lang="ts">
  import type { ImportPreview, ImportResult } from "../../api/imports";

  export let preview: ImportPreview | null = null;
  export let result: ImportResult | null = null;

  $: rows = preview
    ? [
        { label: "笔记", value: preview.note_count },
        { label: "标签", value: preview.tag_count },
        { label: "附件", value: preview.resource_count },
        { label: "提示", value: preview.warning_count },
      ]
    : result
      ? [
          { label: "已导入", value: result.imported_count },
          { label: "已跳过", value: result.skipped_count },
          { label: "失败", value: result.failed_count },
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
