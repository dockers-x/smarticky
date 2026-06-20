<script lang="ts">
  import type { EditorView } from "@codemirror/view";
  import { onDestroy } from "svelte";
  import type { Note } from "../../api/types";
  import { notesStore } from "../../stores/notes";
  import EditorInspector from "./EditorInspector.svelte";
  import EditorToolbar from "./EditorToolbar.svelte";
  import MarkdownEditor from "./MarkdownEditor.svelte";

  export let note: Note | null = null;

  type SaveStatus = "idle" | "saving" | "saved" | "error";

  let editorView: EditorView | null = null;
  let activeNoteID = "";
  let draftTitle = "";
  let draftContent = "";
  let titleTimer: ReturnType<typeof setTimeout> | null = null;
  let contentTimer: ReturnType<typeof setTimeout> | null = null;
  let saveStatus: SaveStatus = "idle";
  let saveSequence = 0;
  let focusMode = false;

  const statusText: Record<SaveStatus, string> = {
    idle: "",
    saving: "正在保存",
    saved: "已保存",
    error: "保存失败",
  };

  function clearTimer(timer: ReturnType<typeof setTimeout> | null): void {
    if (timer) clearTimeout(timer);
  }

  function resetDraft(nextNote: Note | null): void {
    clearTimer(titleTimer);
    clearTimer(contentTimer);
    titleTimer = null;
    contentTimer = null;
    activeNoteID = nextNote?.id ?? "";
    draftTitle = nextNote?.title ?? "";
    draftContent = nextNote?.content ?? "";
    saveStatus = nextNote ? "saved" : "idle";
  }

  $: if ((note?.id ?? "") !== activeNoteID) {
    resetDraft(note);
  }

  function bindEditorView(view: EditorView): void {
    editorView = view;
  }

  function scheduleTitleSave(value: string): void {
    draftTitle = value;
    clearTimer(titleTimer);
    const noteID = activeNoteID;
    titleTimer = setTimeout(() => {
      void persistDraft(noteID, { title: value });
    }, 500);
  }

  function scheduleContentSave(value: string): void {
    draftContent = value;
    clearTimer(contentTimer);
    const noteID = activeNoteID;
    contentTimer = setTimeout(() => {
      void persistDraft(noteID, { content: value });
    }, 500);
  }

  async function persistDraft(
    noteID: string,
    fields: Partial<
      Pick<Note, "title" | "content" | "color" | "is_starred" | "is_deleted">
    >,
  ): Promise<void> {
    if (!noteID || noteID !== activeNoteID) return;

    const sequence = ++saveSequence;
    saveStatus = "saving";

    try {
      await notesStore.updateSelected(fields);
      if (sequence === saveSequence) {
        saveStatus = "saved";
      }
    } catch {
      if (sequence === saveSequence) {
        saveStatus = "error";
      }
    }
  }

  onDestroy(() => {
    clearTimer(titleTimer);
    clearTimer(contentTimer);
  });
</script>

<section class:focus-mode={focusMode} class="editor-pane" aria-label="编辑器">
  {#if note}
    <header class="editor-header">
      <EditorToolbar view={editorView} />
      <div class="editor-header__right">
        <span class:visible={saveStatus !== "idle"} class="editor-save-status">
          {statusText[saveStatus]}
        </span>
        <button
          class="editor-focus-toggle"
          type="button"
          aria-pressed={focusMode}
          on:click={() => (focusMode = !focusMode)}
        >
          {focusMode ? "退出" : "专注"}
        </button>
      </div>
    </header>
    <div class="editor-main">
      <div class="editor-surface">
        <input
          class="editor-title-input"
          value={draftTitle}
          placeholder="未命名"
          aria-label="笔记标题"
          on:input={(event) => scheduleTitleSave(event.currentTarget.value)}
        />
        <MarkdownEditor
          value={draftContent}
          onChange={scheduleContentSave}
          bindView={bindEditorView}
        />
      </div>
      {#if !focusMode}
        <EditorInspector {note} />
      {/if}
    </div>
  {:else}
    <div class="editor-empty">
      <p class="editor-empty-text">选择一篇笔记，或新建一篇</p>
    </div>
  {/if}
</section>
