<script lang="ts">
  import { CrepeBuilder } from "@milkdown/crepe/builder";
  import { blockEdit } from "@milkdown/crepe/feature/block-edit";
  import { codeMirror } from "@milkdown/crepe/feature/code-mirror";
  import { cursor } from "@milkdown/crepe/feature/cursor";
  import { imageBlock } from "@milkdown/crepe/feature/image-block";
  import { latex } from "@milkdown/crepe/feature/latex";
  import { linkTooltip } from "@milkdown/crepe/feature/link-tooltip";
  import { listItem } from "@milkdown/crepe/feature/list-item";
  import { placeholder } from "@milkdown/crepe/feature/placeholder";
  import { table } from "@milkdown/crepe/feature/table";
  import { toolbar } from "@milkdown/crepe/feature/toolbar";
  import { oneDark } from "@codemirror/theme-one-dark";
  import { insert, replaceAll } from "@milkdown/kit/utils";
  import { onDestroy, onMount, tick } from "svelte";
  import type { MarkdownEditorHandle } from "../../editor/markdown";
  import { createEditorDiagramCodeBlockConfig } from "../../markdown/diagrams/editorPreview";
  import { preferencesStore, t } from "../../stores/preferences";

  export let value = "";
  export let onChange: (value: string) => void = () => {};
  export let bindEditor: (editor: MarkdownEditorHandle | null) => void = () => {};

  let host: HTMLDivElement;
  let crepe: CrepeBuilder | null = null;
  let applyingExternalValue = false;
  let lastMarkdown = value;
  let activePreviewTheme = "light";

  const handle: MarkdownEditorHandle = {
    insertMarkdown(markdown: string, inline = false): void {
      if (!crepe) return;
      crepe.editor.action(insert(markdown, inline));
      focusEditor();
    },
    focus(): void {
      focusEditor();
    },
  };

  function focusEditor(): void {
    host?.querySelector<HTMLElement>(".ProseMirror")?.focus();
  }

  async function setMarkdown(nextValue: string): Promise<void> {
    if (!crepe || nextValue === lastMarkdown) return;

    applyingExternalValue = true;
    crepe.editor.action(replaceAll(nextValue, true));
    lastMarkdown = nextValue;
    await tick();
    applyingExternalValue = false;
  }

  async function refreshPreviewTheme(): Promise<void> {
    if (!crepe) return;

    applyingExternalValue = true;
    crepe.editor.action(replaceAll(lastMarkdown, true));
    await tick();
    applyingExternalValue = false;
  }

  onMount(async () => {
    activePreviewTheme = $preferencesStore.theme;
    crepe = new CrepeBuilder({
      root: host,
      defaultValue: value,
    })
      .addFeature(cursor)
      .addFeature(listItem)
      .addFeature(linkTooltip)
      .addFeature(imageBlock)
      .addFeature(blockEdit)
      .addFeature(placeholder, {
        mode: "doc",
        text: t("contentEmpty", $preferencesStore.language),
      })
      .addFeature(toolbar)
      .addFeature(codeMirror, {
        ...createEditorDiagramCodeBlockConfig({
          getTheme: () => ($preferencesStore.theme === "dark" ? "dark" : "light"),
        }),
        theme: oneDark,
      })
      .addFeature(table)
      .addFeature(latex);

    crepe.on((listener) => {
      listener.markdownUpdated((_ctx, markdown) => {
        lastMarkdown = markdown;
        if (!applyingExternalValue) {
          onChange(markdown);
        }
      });
    });

    await crepe.create();
    lastMarkdown = value;
    bindEditor(handle);
  });

  $: if (crepe && value !== lastMarkdown) {
    void setMarkdown(value);
  }

  $: if (crepe && $preferencesStore.theme !== activePreviewTheme) {
    activePreviewTheme = $preferencesStore.theme;
    void refreshPreviewTheme();
  }

  onDestroy(() => {
    bindEditor(null);
    void crepe?.destroy();
    crepe = null;
  });
</script>

<div class="markdown-editor-host" bind:this={host}></div>
