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
  import { commandsCtx, editorViewCtx } from "@milkdown/kit/core";
  import type { Node as ProseNode } from "@milkdown/kit/prose/model";
  import { clearTextInCurrentBlockCommand } from "@milkdown/kit/preset/commonmark";
  import { forceUpdate, insert, replaceAll } from "@milkdown/kit/utils";
  import { onDestroy, onMount, tick } from "svelte";
  import type { UUID } from "../../api/types";
  import type { MarkdownEditorHandle } from "../../editor/markdown";
  import {
    preserveCodeGroups,
    replaceCodeGroupSource,
    type CodeGroupSourceEditRequest,
  } from "../../markdown/codeGroups";
  import { createCodeGroupEditorPlugin } from "../../markdown/editorCodeGroups";
  import { createEditorTocPlugin } from "../../markdown/editorToc";
  import { createEditorDiagramCodeBlockConfig } from "../../markdown/diagrams/editorPreview";
  import { fetchProtectedImageObjectURL } from "../../markdown/protectedImages";
  import { uploadAttachment } from "../../stores/attachments";
  import { preferencesStore, t } from "../../stores/preferences";

  export let value = "";
  export let noteId: UUID = "";
  export let onChange: (value: string, noteID: UUID) => void = () => {};
  export let bindEditor: (editor: MarkdownEditorHandle | null) => void = () => {};

  const ownerNoteID = noteId;

  let host: HTMLDivElement;
  let crepe: CrepeBuilder | null = null;
  let applyingExternalValue = false;
  let lastMarkdown = value;
  let lastDocument: ProseNode | null = null;
  let activePreviewTheme = "light";
  let pendingCodeGroupSources: string[] = [];
  let editorReady = false;
  let initializing = false;
  let initializationError = "";
  let disposed = false;

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
    get noteID(): string {
      return ownerNoteID;
    },
    getMarkdown(): string {
      return readMarkdown();
    },
    insertMarkdown(markdown: string, inline = false): void {
      if (!crepe) return;
      crepe.editor.action(insert(markdown, inline));
      focusEditor();
    },
    focus(): void {
      focusEditor();
    },
  };

  function readMarkdown(): string {
    if (!crepe || !editorReady) return lastMarkdown;
    const currentDocument = crepe.editor.action((ctx) => ctx.get(editorViewCtx).state.doc);
    if (lastDocument?.eq(currentDocument)) return lastMarkdown;
    const markdown = preserveCodeGroups(crepe.getMarkdown(), lastMarkdown, pendingCodeGroupSources);
    lastDocument = currentDocument;
    lastMarkdown = markdown;
    clearResolvedPendingCodeGroups(markdown);
    return markdown;
  }

  function focusEditor(): void {
    host?.querySelector<HTMLElement>(".ProseMirror")?.focus();
  }

  function updateEditorAccessibility(): void {
    const editor = host?.querySelector<HTMLElement>(".ProseMirror");
    if (!editor) return;
    editor.setAttribute("aria-label", t("noteContent", $preferencesStore.language));
    editor.setAttribute("role", "textbox");
    editor.setAttribute("aria-multiline", "true");
    editor.setAttribute("autocapitalize", "sentences");
  }

  async function setMarkdown(nextValue: string): Promise<void> {
    if (!crepe || !editorReady || nextValue === lastMarkdown) return;

    applyingExternalValue = true;
    crepe.editor.action(replaceAll(nextValue, true));
    lastMarkdown = nextValue;
    lastDocument = crepe.editor.action((ctx) => ctx.get(editorViewCtx).state.doc);
    refreshCodeGroupDecorations();
    await tick();
    applyingExternalValue = false;
  }

  async function refreshPreviewTheme(): Promise<void> {
    if (!crepe || !editorReady) return;

    const markdown = readMarkdown();
    if (markdown !== value) onChange(markdown, noteId);
    applyingExternalValue = true;
    crepe.editor.action(replaceAll(markdown, true));
    lastDocument = crepe.editor.action((ctx) => ctx.get(editorViewCtx).state.doc);
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

  function replaceLocalCodeGroupSource(
    request: CodeGroupSourceEditRequest,
    nextRaw: string,
  ): string | null {
    if (!crepe) return "Editor is not ready.";

    const result = replaceCodeGroupSource(readMarkdown(), request, nextRaw);
    if (result.error) return result.error;

    applyingExternalValue = true;
    crepe.editor.action(replaceAll(result.markdown, true));
    lastMarkdown = result.markdown;
    lastDocument = crepe.editor.action((ctx) => ctx.get(editorViewCtx).state.doc);
    onChange(result.markdown, noteId);
    refreshCodeGroupDecorations();
    void tick().then(() => {
      applyingExternalValue = false;
      focusEditor();
    });
    return null;
  }

  async function uploadEditorImage(file: File): Promise<string> {
    if (!noteId) throw new Error("Note is required before uploading images");
    const attachment = await uploadAttachment(noteId, file);
    return attachment.download_url || `/api/attachments/${attachment.id}/download`;
  }

  async function initializeEditor(): Promise<void> {
    if (initializing || editorReady || disposed) return;
    initializing = true;
    initializationError = "";
    const initialMarkdown = value;
    activePreviewTheme = $preferencesStore.theme;
    crepe = new CrepeBuilder({
      root: host,
      defaultValue: initialMarkdown,
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

    crepe.editor.use(createCodeGroupEditorPlugin(() => lastMarkdown, replaceLocalCodeGroupSource));
    crepe.editor.use(createEditorTocPlugin());

    const editorNoteID = ownerNoteID;
    crepe.on((listener) => {
      listener.markdownUpdated(() => {
        if (disposed || !editorReady) return;
        const previousMarkdown = lastMarkdown;
        const nextMarkdown = readMarkdown();
        if (!applyingExternalValue && nextMarkdown !== previousMarkdown) {
          onChange(nextMarkdown, editorNoteID);
        }
        refreshCodeGroupDecorations();
      });
    });

    const instance = crepe;
    try {
      await instance.create();
      if (disposed) {
        await instance.destroy();
        return;
      }
      editorReady = true;
      lastMarkdown = initialMarkdown;
      lastDocument = instance.editor.action((ctx) => ctx.get(editorViewCtx).state.doc);
      await setMarkdown(value);
      if (disposed) return;
      updateEditorAccessibility();
      refreshCodeGroupDecorations();
      bindEditor(handle);
    } catch {
      if (!disposed) {
        initializationError = $preferencesStore.language === "zh"
          ? "编辑器暂时无法载入。可以重试，或使用上方的 Markdown 源码模式继续编辑。"
          : "The editor could not load. Retry, or continue in Markdown source mode using the toolbar.";
        editorReady = false;
        bindEditor(null);
      }
      await instance.destroy().catch(() => {});
      if (crepe === instance) crepe = null;
    } finally {
      initializing = false;
    }
  }

  onMount(() => {
    void initializeEditor();
  });

  $: if (editorReady && crepe && value !== lastMarkdown) {
    void setMarkdown(value);
  }

  $: if (editorReady && crepe && $preferencesStore.theme !== activePreviewTheme) {
    activePreviewTheme = $preferencesStore.theme;
    void refreshPreviewTheme();
  }

  $: if (editorReady && crepe && $preferencesStore.language) {
    updateEditorAccessibility();
  }

  onDestroy(() => {
    disposed = true;
    bindEditor(null);
    if (editorReady) void crepe?.destroy().catch(() => {});
    crepe = null;
  });
</script>

<div class="markdown-editor-shell">
  <div class="markdown-editor-host" bind:this={host} aria-busy={initializing} data-ready={editorReady}></div>
  {#if initializing}
    <p class="markdown-editor-state" role="status" aria-live="polite">{t("loading", $preferencesStore.language)}</p>
  {:else if initializationError}
    <div class="markdown-editor-state markdown-editor-state--error" role="alert">
      <p>{initializationError}</p>
      <button type="button" on:click={() => void initializeEditor()}>
        {$preferencesStore.language === "zh" ? "重新载入编辑器" : "Reload editor"}
      </button>
    </div>
  {/if}
</div>

<style>
  .markdown-editor-shell {
    position: relative;
    min-width: 0;
  }

  .markdown-editor-host[data-ready="false"] {
    visibility: hidden;
  }

  .markdown-editor-state {
    position: absolute;
    inset: 0 0 auto;
    margin: 0;
    color: var(--color-text-sub);
    font: 0.8125rem/1.65 var(--font-ui);
  }

  .markdown-editor-state--error {
    border: 1px solid var(--color-divider);
    border-radius: var(--radius-control);
    background: var(--color-surface-secondary);
    padding: 1rem;
  }

  .markdown-editor-state--error p {
    margin: 0 0 0.75rem;
  }

  .markdown-editor-state--error button {
    min-height: 36px;
    border: 1px solid var(--color-divider);
    border-radius: var(--radius-control);
    background: var(--color-card);
    color: var(--color-text);
    padding: 0.375rem 0.75rem;
  }

  @media (pointer: coarse) {
    .markdown-editor-state--error button {
      min-height: 44px;
    }
  }
</style>
