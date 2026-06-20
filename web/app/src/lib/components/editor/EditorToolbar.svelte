<script lang="ts">
  import type { EditorView } from "@codemirror/view";
  import {
    insertTask,
    prefixLine,
    wrapSelection,
  } from "../../editor/commands";
  import { preferencesStore, t } from "../../stores/preferences";

  export let view: EditorView | null = null;

  $: actions = [
    {
      label: "B",
      title: t("bold", $preferencesStore.language),
      run: () => view && wrapSelection(view, "**"),
    },
    {
      label: "I",
      title: t("italic", $preferencesStore.language),
      run: () => view && wrapSelection(view, "*"),
    },
    {
      label: "•",
      title: t("unorderedList", $preferencesStore.language),
      run: () => view && prefixLine(view, "- "),
    },
    {
      label: "1.",
      title: t("orderedList", $preferencesStore.language),
      run: () => view && prefixLine(view, "1. "),
    },
    {
      label: "☐",
      title: t("task", $preferencesStore.language),
      run: () => view && insertTask(view),
    },
  ];
</script>

<div class="editor-toolbar" role="toolbar" aria-label={t("markdownToolbar", $preferencesStore.language)}>
  {#each actions as action}
    <button
      type="button"
      aria-label={action.title}
      title={action.title}
      disabled={!view}
      on:click={action.run}
    >
      {action.label}
    </button>
  {/each}
</div>
