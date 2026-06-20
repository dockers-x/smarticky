<script lang="ts">
  import type { EditorView } from "@codemirror/view";
  import { onDestroy, tick } from "svelte";
  import type { Note } from "../../api/types";
  import { confirmDialog, notify } from "../../stores/dialogs";
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
  let titleInput: HTMLTextAreaElement | null = null;
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
    void tick().then(resizeTitleInput);
  }

  $: if ((note?.id ?? "") !== activeNoteID) {
    resetDraft(note);
  }

  function bindEditorView(view: EditorView): void {
    editorView = view;
  }

  function scheduleTitleSave(value: string): void {
    draftTitle = value;
    resizeTitleInput();
    clearTimer(titleTimer);
    const noteID = activeNoteID;
    titleTimer = setTimeout(() => {
      void persistDraft(noteID, { title: value });
    }, 500);
  }

  function resizeTitleInput(): void {
    if (!titleInput) return;
    titleInput.style.height = "auto";
    titleInput.style.height = `${titleInput.scrollHeight}px`;
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

  async function toggleStar(): Promise<void> {
    if (!note) return;

    try {
      await notesStore.updateSelected({ is_starred: !note.is_starred });
      notify(note.is_starred ? "已取消收藏" : "已收藏", "success");
    } catch {
      notify("更新收藏状态失败", "error");
    }
  }

  async function toggleTrash(): Promise<void> {
    if (!note) return;

    const restoring = note.is_deleted;
    const confirmed = await confirmDialog({
      title: restoring ? "恢复笔记" : "移入废纸篓",
      message: restoring
        ? "确认将这篇笔记恢复到普通列表？"
        : "确认将这篇笔记移入废纸篓？",
      confirmLabel: restoring ? "恢复" : "移入",
      cancelLabel: "取消",
    });
    if (!confirmed) return;

    try {
      await notesStore.updateSelected({ is_deleted: !restoring });
      notesStore.clearSelection();
      await notesStore.load();
      notify(restoring ? "已恢复笔记" : "已移入废纸篓", "success");
    } catch {
      notify(restoring ? "恢复失败" : "移动失败", "error");
    }
  }

  onDestroy(() => {
    clearTimer(titleTimer);
    clearTimer(contentTimer);
  });
</script>

<section
  class:focus-mode={focusMode}
  class:has-note={Boolean(note)}
  class="editor-pane"
  aria-label="编辑器"
>
  {#if note}
    <header class="editor-header">
      <button
        class="editor-mobile-back"
        type="button"
        aria-label="返回笔记列表"
        on:click={() => notesStore.clearSelection()}
      >
        ‹
      </button>
      <EditorToolbar view={editorView} />
      <div class="editor-header__right">
        <span class:visible={saveStatus !== "idle"} class="editor-save-status">
          {statusText[saveStatus]}
        </span>
        <button
          class="editor-action-button"
          type="button"
          aria-pressed={note.is_starred}
          on:click={toggleStar}
        >
          {note.is_starred ? "已收藏" : "收藏"}
        </button>
        <button class="editor-action-button danger" type="button" on:click={toggleTrash}>
          {note.is_deleted ? "恢复" : "删除"}
        </button>
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
        <textarea
          bind:this={titleInput}
          class="editor-title-input"
          value={draftTitle}
          placeholder="未命名"
          aria-label="笔记标题"
          rows="1"
          on:input={(event) => scheduleTitleSave(event.currentTarget.value)}
        ></textarea>
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
