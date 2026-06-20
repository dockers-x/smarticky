<script lang="ts">
  import type { EditorView } from "@codemirror/view";
  import {
    insertImage,
    insertTask,
    prefixLine,
    wrapSelection,
  } from "../../editor/commands";

  export let view: EditorView | null = null;

  const actions = [
    {
      label: "B",
      title: "加粗",
      run: () => view && wrapSelection(view, "**"),
    },
    {
      label: "I",
      title: "斜体",
      run: () => view && wrapSelection(view, "*"),
    },
    {
      label: "•",
      title: "无序列表",
      run: () => view && prefixLine(view, "- "),
    },
    {
      label: "1.",
      title: "有序列表",
      run: () => view && prefixLine(view, "1. "),
    },
    {
      label: "☐",
      title: "待办",
      run: () => view && insertTask(view),
    },
    {
      label: "图",
      title: "插入图片",
      run: () => view && insertImage(view),
    },
  ];
</script>

<div class="editor-toolbar" role="toolbar" aria-label="Markdown 工具栏">
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
