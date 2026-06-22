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
  import { commandsCtx } from "@milkdown/kit/core";
  import { clearTextInCurrentBlockCommand } from "@milkdown/kit/preset/commonmark";
  import { forceUpdate, insert, replaceAll } from "@milkdown/kit/utils";
  import { onDestroy, onMount, tick } from "svelte";
  import type { UUID } from "../../api/types";
  import type { MarkdownEditorHandle } from "../../editor/markdown";
  import { preserveCodeGroups } from "../../markdown/codeGroups";
  import { createCodeGroupEditorPlugin } from "../../markdown/editorCodeGroups";
  import { createEditorDiagramCodeBlockConfig } from "../../markdown/diagrams/editorPreview";
  import { fetchProtectedImageObjectURL } from "../../markdown/protectedImages";
  import { uploadAttachment } from "../../stores/attachments";
  import { preferencesStore, t } from "../../stores/preferences";

  export let value = "";
  export let noteId: UUID = "";
  export let onChange: (value: string) => void = () => {};
  export let bindEditor: (editor: MarkdownEditorHandle | null) => void = () => {};
  export let requestSourceMode: () => void = () => {};

  let host: HTMLDivElement;
  let crepe: CrepeBuilder | null = null;
  let applyingExternalValue = false;
  let lastMarkdown = value;
  let activePreviewTheme = "light";
  let pendingCodeGroupSources: string[] = [];

  const codeGroupTemplate = [
    "::: code-group",
    "```bash [pnpm]",
    "pnpm install",
    "```",
    "```bash [yarn]",
    "yarn install",
    "```",
    "```bash [npm]",
    "npm install",
    "```",
    ":::",
    "",
  ].join("\n");

  const codeGroupIcon = `
    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="currentColor">
      <path d="M4 5.5A2.5 2.5 0 0 1 6.5 3h11A2.5 2.5 0 0 1 20 5.5v13a2.5 2.5 0 0 1-2.5 2.5h-11A2.5 2.5 0 0 1 4 18.5v-13Zm2.5-.5a.5.5 0 0 0-.5.5V8h12V5.5a.5.5 0 0 0-.5-.5h-11ZM6 10v8.5a.5.5 0 0 0 .5.5h11a.5.5 0 0 0 .5-.5V10H6Z"/>
      <path d="M8 6h3v1.5H8V6Zm4.5 0H16v1.5h-3.5V6Zm-2.8 10.8L7.4 14.5l2.3-2.3 1.1 1.1-1.2 1.2 1.2 1.2-1.1 1.1Zm4.6 0-1.1-1.1 1.2-1.2-1.2-1.2 1.1-1.1 2.3 2.3-2.3 2.3Z"/>
    </svg>
  `;

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
    refreshCodeGroupDecorations();
    await tick();
    applyingExternalValue = false;
  }

  async function refreshPreviewTheme(): Promise<void> {
    if (!crepe) return;

    applyingExternalValue = true;
    crepe.editor.action(replaceAll(lastMarkdown, true));
    refreshCodeGroupDecorations();
    await tick();
    applyingExternalValue = false;
  }

  function refreshCodeGroupDecorations(): void {
    crepe?.editor.action(forceUpdate());
  }

  function rememberPendingCodeGroupSource(markdown: string): void {
    pendingCodeGroupSources = [...pendingCodeGroupSources, markdown.trim()];
  }

  function clearResolvedPendingCodeGroups(markdown: string): void {
    pendingCodeGroupSources = pendingCodeGroupSources.filter(
      (source) => !markdown.includes(source),
    );
  }

  async function uploadEditorImage(file: File): Promise<string> {
    if (!noteId) throw new Error("Note is required before uploading images");
    const attachment = await uploadAttachment(noteId, file);
    return attachment.download_url || `/api/attachments/${attachment.id}/download`;
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
      .addFeature(imageBlock, {
        onUpload: uploadEditorImage,
        proxyDomURL: fetchProtectedImageObjectURL,
      })
      .addFeature(blockEdit, {
        buildMenu(builder) {
          const advanced = builder.getGroup("advanced");
          advanced.addItem("code-group", {
            label: "Code Group",
            icon: codeGroupIcon,
            onRun(ctx) {
              rememberPendingCodeGroupSource(codeGroupTemplate);
              ctx.get(commandsCtx).call(clearTextInCurrentBlockCommand.key);
              insert(codeGroupTemplate)(ctx);
            },
          });
          const codeGroupItem = advanced.group.items.pop();
          const codeIndex = advanced.group.items.findIndex((item) => item.key === "code");
          if (codeGroupItem) {
            advanced.group.items.splice(codeIndex >= 0 ? codeIndex + 1 : advanced.group.items.length, 0, codeGroupItem);
          }
        },
      })
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

    crepe.editor.use(createCodeGroupEditorPlugin(() => lastMarkdown, requestSourceMode));

    crepe.on((listener) => {
      listener.markdownUpdated((_ctx, markdown) => {
        const nextMarkdown = preserveCodeGroups(
          markdown,
          lastMarkdown,
          pendingCodeGroupSources,
        );
        lastMarkdown = nextMarkdown;
        clearResolvedPendingCodeGroups(nextMarkdown);
        if (!applyingExternalValue) {
          onChange(nextMarkdown);
        }
        refreshCodeGroupDecorations();
      });
    });

    await crepe.create();
    lastMarkdown = value;
    refreshCodeGroupDecorations();
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
