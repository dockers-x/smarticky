<script lang="ts">
  import { Crepe } from "@milkdown/crepe";
  import { insert, replaceAll } from "@milkdown/kit/utils";
  import { onDestroy, onMount, tick } from "svelte";
  import type { MarkdownEditorHandle } from "../../editor/markdown";
  import { preferencesStore, t } from "../../stores/preferences";

  export let value = "";
  export let onChange: (value: string) => void = () => {};
  export let bindEditor: (editor: MarkdownEditorHandle | null) => void = () => {};

  let host: HTMLDivElement;
  let crepe: Crepe | null = null;
  let applyingExternalValue = false;
  let lastMarkdown = value;

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

  onMount(async () => {
    crepe = new Crepe({
      root: host,
      defaultValue: value,
      featureConfigs: {
        [Crepe.Feature.Placeholder]: {
          mode: "doc",
          text: t("contentEmpty", $preferencesStore.language),
        },
      },
    });

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

  onDestroy(() => {
    bindEditor(null);
    void crepe?.destroy();
    crepe = null;
  });
</script>

<div class="markdown-editor-host" bind:this={host}></div>
