<script lang="ts">
  import type { EditorView } from "@codemirror/view";
  import { onDestroy, tick } from "svelte";
  import type { Note } from "../../api/types";
  import { confirmDialog, notify } from "../../stores/dialogs";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";
  import EditorInspector from "./EditorInspector.svelte";
  import EditorToolbar from "./EditorToolbar.svelte";
  import MarkdownEditor from "./MarkdownEditor.svelte";
  import ShareImageDialog from "./ShareImageDialog.svelte";

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
  let detailsOpen = false;
  let shareOpen = false;

  $: statusText = {
    idle: "",
    saving: t("saving", $preferencesStore.language),
    saved: t("saved", $preferencesStore.language),
    error: t("saveError", $preferencesStore.language),
  } satisfies Record<SaveStatus, string>;

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
    detailsOpen = false;
    shareOpen = false;
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
      notify(
        note.is_starred
          ? t("starRemoved", $preferencesStore.language)
          : t("starAdded", $preferencesStore.language),
        "success",
      );
    } catch {
      notify(t("updateStarFailed", $preferencesStore.language), "error");
    }
  }

  async function toggleTrash(): Promise<void> {
    if (!note) return;

    const restoring = note.is_deleted;
    const confirmed = await confirmDialog({
      title: restoring
        ? t("restoreNote", $preferencesStore.language)
        : t("trashNote", $preferencesStore.language),
      message: restoring
        ? t("restoreNoteMessage", $preferencesStore.language)
        : t("trashNoteMessage", $preferencesStore.language),
      confirmLabel: restoring
        ? t("restore", $preferencesStore.language)
        : t("trashNote", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
    });
    if (!confirmed) return;

    try {
      await notesStore.updateSelected({ is_deleted: !restoring });
      notesStore.clearSelection();
      await notesStore.load();
      notify(
        restoring
          ? t("restoredNote", $preferencesStore.language)
          : t("trashedNote", $preferencesStore.language),
        "success",
      );
    } catch {
      notify(
        restoring
          ? t("restoreFailed", $preferencesStore.language)
          : t("trashFailed", $preferencesStore.language),
        "error",
      );
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
  aria-label={t("editor", $preferencesStore.language)}
>
  {#if note}
    <header class="editor-header">
      <button
        class="editor-mobile-back"
        type="button"
        aria-label={t("back", $preferencesStore.language)}
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
          {note.is_starred
            ? t("starAdded", $preferencesStore.language)
            : t("star", $preferencesStore.language)}
        </button>
        <button class="editor-action-button danger" type="button" on:click={toggleTrash}>
          {note.is_deleted
            ? t("restore", $preferencesStore.language)
            : t("delete", $preferencesStore.language)}
        </button>
        <button
          class="editor-action-button"
          type="button"
          aria-pressed={detailsOpen}
          on:click={() => (detailsOpen = !detailsOpen)}
        >
          {t("showDetails", $preferencesStore.language)}
        </button>
        <button
          class="editor-focus-toggle"
          type="button"
          aria-pressed={focusMode}
          on:click={() => (focusMode = !focusMode)}
        >
          {focusMode
            ? t("exitFocus", $preferencesStore.language)
            : t("enterFocus", $preferencesStore.language)}
        </button>
        <button class="editor-share-button" type="button" on:click={() => (shareOpen = true)}>
          {t("generateImage", $preferencesStore.language)}
        </button>
      </div>
    </header>
    <div class:details-open={detailsOpen && !focusMode} class="editor-main">
      <div class="editor-surface">
        <textarea
          bind:this={titleInput}
          class="editor-title-input"
          value={draftTitle}
          placeholder={t("untitled", $preferencesStore.language)}
          aria-label={t("noteTitle", $preferencesStore.language)}
          rows="1"
          on:input={(event) => scheduleTitleSave(event.currentTarget.value)}
        ></textarea>
        <MarkdownEditor
          value={draftContent}
          onChange={scheduleContentSave}
          bindView={bindEditorView}
        />
      </div>
      {#if detailsOpen && !focusMode}
        <EditorInspector {note} />
      {/if}
    </div>
    {#if shareOpen}
      <ShareImageDialog
        title={draftTitle}
        content={draftContent}
        onClose={() => (shareOpen = false)}
      />
    {/if}
  {:else}
    <div class="editor-empty">
      <p class="editor-empty-text">{t("selectOrCreate", $preferencesStore.language)}</p>
    </div>
  {/if}
</section>
