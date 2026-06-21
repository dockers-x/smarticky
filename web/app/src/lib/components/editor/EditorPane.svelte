<script lang="ts">
  import { FileCode, Image as ImageIcon } from "@lucide/svelte";
  import { onDestroy, onMount, tick } from "svelte";
  import type { Note } from "../../api/types";
  import type { MarkdownEditorHandle } from "../../editor/markdown";
  import { authStore } from "../../stores/auth";
  import { confirmDialog, notify } from "../../stores/dialogs";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";
  import { tagsStore } from "../../stores/tags";
  import EditorInspector from "./EditorInspector.svelte";
  import MarkdownEditor from "./MarkdownEditor.svelte";
  import ShareImageDialog from "./ShareImageDialog.svelte";

  export let note: Note | null = null;

  type SaveStatus = "idle" | "saving" | "saved" | "error";

  let markdownEditor: MarkdownEditorHandle | null = null;
  let activeNoteID = "";
  let draftTitle = "";
  let draftContent = "";
  let titleInput: HTMLTextAreaElement | null = null;
  let sourceTextarea: HTMLTextAreaElement | null = null;
  let titleTimer: ReturnType<typeof setTimeout> | null = null;
  let contentTimer: ReturnType<typeof setTimeout> | null = null;
  let saveStatus: SaveStatus = "idle";
  let saveSequence = 0;
  let sourceMode = false;
  let focusMode = false;
  let detailsOpen = false;
  let shareOpen = false;
  let actionMenuOpen = false;
  let folderMenuOpen = false;
  let quickTagName = "";
  let tagBusy = false;

  $: statusText = {
    idle: "",
    saving: t("saving", $preferencesStore.language),
    saved: t("saved", $preferencesStore.language),
    error: t("saveError", $preferencesStore.language),
  } satisfies Record<SaveStatus, string>;

  $: noteDate = note
    ? new Date(note.updated_at).toLocaleDateString(
        $preferencesStore.language === "zh" ? "zh-CN" : "en-US",
        {
          year: "numeric",
          month: "2-digit",
          day: "2-digit",
        },
      )
    : "";
  $: wordCount = draftContent.replace(/\s/g, "").length;
  $: currentTagNames = note?.tags?.map((tag) => tag.name) ?? [];
  $: folderLabel = currentTagNames[0] || t("allNotes", $preferencesStore.language);
  $: availableTags = $tagsStore.filter(
    (tag) =>
      !currentTagNames.some(
        (name) => name.toLowerCase() === tag.name.toLowerCase(),
      ),
  );

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
    sourceMode = false;
    detailsOpen = false;
    shareOpen = false;
    actionMenuOpen = false;
    folderMenuOpen = false;
    quickTagName = "";
    void tick().then(resizeTitleInput);
  }

  $: if ((note?.id ?? "") !== activeNoteID) {
    resetDraft(note);
  }

  function bindMarkdownEditor(editor: MarkdownEditorHandle | null): void {
    markdownEditor = editor;
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

  function resizeSourceInput(): void {
    if (!sourceTextarea) return;
    sourceTextarea.style.height = "auto";
    sourceTextarea.style.height = `${sourceTextarea.scrollHeight}px`;
  }

  function scheduleContentSave(value: string): void {
    draftContent = value;
    void tick().then(resizeSourceInput);
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

  async function flushDraft(): Promise<void> {
    if (!note || !activeNoteID) return;

    const fields: Partial<
      Pick<Note, "title" | "content" | "color" | "is_starred" | "is_deleted">
    > = {};
    if (draftTitle !== note.title) fields.title = draftTitle;
    if (draftContent !== note.content) fields.content = draftContent;

    clearTimer(titleTimer);
    clearTimer(contentTimer);
    titleTimer = null;
    contentTimer = null;

    if (Object.keys(fields).length > 0) {
      await persistDraft(activeNoteID, fields);
    }
  }

  async function finishEditing(): Promise<void> {
    try {
      await flushDraft();
    } finally {
      notesStore.clearSelection();
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

  function runImageInsert(): void {
    const markdown = `![${t("imageInsertAlt", $preferencesStore.language)}]()`;
    if (sourceMode) {
      insertIntoSource(markdown);
      return;
    }
    if (!markdownEditor) return;
    markdownEditor.insertMarkdown(markdown, true);
  }

  function insertIntoSource(markdown: string): void {
    if (!sourceTextarea) {
      scheduleContentSave(`${draftContent}${markdown}`);
      return;
    }

    const start = sourceTextarea.selectionStart ?? draftContent.length;
    const end = sourceTextarea.selectionEnd ?? start;
    const nextValue = `${draftContent.slice(0, start)}${markdown}${draftContent.slice(end)}`;
    scheduleContentSave(nextValue);
    void tick().then(() => {
      const cursor = start + markdown.length;
      sourceTextarea?.focus();
      sourceTextarea?.setSelectionRange(cursor, cursor);
      resizeSourceInput();
    });
  }

  function toggleSourceMode(): void {
    sourceMode = !sourceMode;
    actionMenuOpen = false;
    void tick().then(() => {
      if (sourceMode) {
        resizeSourceInput();
        sourceTextarea?.focus();
      } else {
        markdownEditor?.focus();
      }
    });
  }

  function handleEditorKeydown(event: KeyboardEvent): void {
    if (!note || shareOpen) return;
    if (!(event.metaKey || event.ctrlKey) || event.shiftKey) return;
    if (event.key !== "/" && event.code !== "Slash") return;

    event.preventDefault();
    toggleSourceMode();
  }

  async function addQuickTag(name = quickTagName): Promise<void> {
    if (!note || tagBusy) return;
    const trimmed = name.trim();
    if (!trimmed) return;
    if (
      note.tags?.some((tag) => tag.name.toLowerCase() === trimmed.toLowerCase())
    ) {
      quickTagName = "";
      return;
    }

    tagBusy = true;
    try {
      await tagsStore.addToNote(note.id, trimmed);
      await notesStore.load();
      quickTagName = "";
      folderMenuOpen = false;
    } catch {
      notify(t("addTagFailed", $preferencesStore.language), "error");
    } finally {
      tagBusy = false;
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

  async function deletePermanent(): Promise<void> {
    if (!note) return;

    const confirmed = await confirmDialog({
      title: t("deleteForever", $preferencesStore.language),
      message: t("deleteForeverMessage", $preferencesStore.language),
      confirmLabel: t("deleteForever", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
    });
    if (!confirmed) return;

    try {
      await notesStore.deletePermanent(note.id);
      notify(t("deletedNote", $preferencesStore.language), "success");
    } catch {
      notify(t("deleteForeverFailed", $preferencesStore.language), "error");
    }
  }

  function closeActionMenu(): void {
    actionMenuOpen = false;
  }

  async function runMenuAction(action: () => void | Promise<void>): Promise<void> {
    closeActionMenu();
    await action();
  }

  onMount(() => {
    void tagsStore.load();
  });

  onDestroy(() => {
    clearTimer(titleTimer);
    clearTimer(contentTimer);
  });
</script>

<svelte:window on:keydown={handleEditorKeydown} />

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
        on:click={() => void finishEditing()}
      >
        ‹
      </button>
      <div class="editor-header__right">
        <span class:visible={saveStatus !== "idle"} class="editor-save-status">
          {statusText[saveStatus]}
        </span>
        <button
          class="editor-icon-button"
          type="button"
          title={t("insertImage", $preferencesStore.language)}
          aria-label={t("insertImage", $preferencesStore.language)}
          disabled={!markdownEditor && !sourceMode}
          on:click={runImageInsert}
        >
          <ImageIcon aria-hidden="true" size={19} strokeWidth={2} />
        </button>
        <button
          class="editor-icon-button"
          type="button"
          title={sourceMode
            ? t("wysiwygMode", $preferencesStore.language)
            : t("sourceMode", $preferencesStore.language)}
          aria-label={sourceMode
            ? t("wysiwygMode", $preferencesStore.language)
            : t("sourceMode", $preferencesStore.language)}
          aria-pressed={sourceMode}
          on:click={toggleSourceMode}
        >
          <FileCode aria-hidden="true" size={19} strokeWidth={2} />
        </button>
        {#if note.is_deleted}
          <button class="editor-action-button" type="button" on:click={toggleTrash}>
            {t("restore", $preferencesStore.language)}
          </button>
          <button class="editor-action-button danger" type="button" on:click={deletePermanent}>
            <span class="editor-action-label">{t("deleteForever", $preferencesStore.language)}</span>
            <span class="editor-action-label--compact">{t("deleteForeverShort", $preferencesStore.language)}</span>
          </button>
        {:else}
          <button class="editor-share-button" type="button" on:click={() => (shareOpen = true)}>
            <span class="editor-share-label">{t("generateImage", $preferencesStore.language)}</span>
            <span class="editor-share-label--compact">{t("generateImageShort", $preferencesStore.language)}</span>
          </button>
        {/if}
        <button
          class="editor-done-button"
          type="button"
          aria-label={t("done", $preferencesStore.language)}
          title={t("done", $preferencesStore.language)}
          on:click={() => void finishEditing()}
        >
          ✓
        </button>
        <div class="editor-action-menu">
          <button
            class="editor-more-button"
            type="button"
            aria-label={t("moreActions", $preferencesStore.language)}
            aria-expanded={actionMenuOpen}
            title={t("moreActions", $preferencesStore.language)}
            on:click={() => (actionMenuOpen = !actionMenuOpen)}
          >
            ⋯
          </button>
          {#if actionMenuOpen}
            <div class="editor-action-menu__content">
              <button
                class="editor-action-button"
                type="button"
                aria-pressed={detailsOpen}
                on:click={() =>
                  void runMenuAction(() => {
                    detailsOpen = !detailsOpen;
                  })}
              >
                {t("showDetails", $preferencesStore.language)}
              </button>
              <button
                class="editor-action-button"
                type="button"
                aria-pressed={focusMode}
                on:click={() =>
                  void runMenuAction(() => {
                    focusMode = !focusMode;
                  })}
              >
                {focusMode
                  ? t("exitFocus", $preferencesStore.language)
                  : t("enterFocus", $preferencesStore.language)}
              </button>
              {#if note.is_deleted}
                <button
                  class="editor-action-button"
                  type="button"
                  on:click={() =>
                    void runMenuAction(() => {
                      shareOpen = true;
                    })}
                >
                  {t("generateImage", $preferencesStore.language)}
                </button>
              {:else}
                <button
                  class="editor-action-button editor-menu-share"
                  type="button"
                  on:click={() =>
                    void runMenuAction(() => {
                      shareOpen = true;
                    })}
                >
                  {t("generateImage", $preferencesStore.language)}
                </button>
                <button
                  class="editor-action-button danger"
                  type="button"
                  on:click={() => void runMenuAction(toggleTrash)}
                >
                  {t("trashNote", $preferencesStore.language)}
                </button>
              {/if}
            </div>
          {/if}
        </div>
      </div>
    </header>
    <div class="editor-meta-bar">
      <div class="editor-meta-popover-anchor">
        <button
          class="editor-meta-button"
          type="button"
          aria-label={`${t("noteLocation", $preferencesStore.language)}: ${folderLabel}`}
          aria-expanded={folderMenuOpen}
          on:click={() => {
            folderMenuOpen = !folderMenuOpen;
          }}
        >
          <span>{folderLabel}</span>
          <span aria-hidden="true">⌄</span>
        </button>
        {#if folderMenuOpen}
          <div class="editor-popover editor-folder-popover">
            <p class="editor-popover-title">{t("assignTag", $preferencesStore.language)}</p>
            <button class="editor-popover-row active" type="button" disabled>
              <span>{t("allNotes", $preferencesStore.language)}</span>
              <span aria-hidden="true">✓</span>
            </button>
            {#if currentTagNames.length}
              <p class="editor-popover-label">{t("currentTags", $preferencesStore.language)}</p>
              {#each currentTagNames as tagName}
                <button class="editor-popover-row active" type="button" disabled>
                  <span>{tagName}</span>
                  <span aria-hidden="true">✓</span>
                </button>
              {/each}
            {/if}
            {#if availableTags.length}
              <p class="editor-popover-label">{t("availableTags", $preferencesStore.language)}</p>
              {#each availableTags as tag (tag.id)}
                <button
                  class="editor-popover-row"
                  type="button"
                  disabled={tagBusy}
                  on:click={() => void addQuickTag(tag.name)}
                >
                  <span>{tag.name}</span>
                  <span aria-hidden="true">＋</span>
                </button>
              {/each}
            {/if}
            <form class="editor-popover-form" on:submit|preventDefault={() => void addQuickTag()}>
              <input
                bind:value={quickTagName}
                type="text"
                placeholder={t("addTag", $preferencesStore.language)}
                aria-label={t("addTag", $preferencesStore.language)}
                disabled={tagBusy}
              />
              <button type="submit" disabled={tagBusy || !quickTagName.trim()}>
                {t("add", $preferencesStore.language)}
              </button>
            </form>
          </div>
        {/if}
      </div>
      <time class="editor-meta-text" datetime={note.updated_at}>{noteDate}</time>
      <span class="editor-meta-text">{wordCount} {t("wordUnit", $preferencesStore.language)}</span>
      <span class="editor-meta-spacer"></span>
      <button
        class="editor-meta-star"
        type="button"
        title={note.is_starred
          ? t("starRemoved", $preferencesStore.language)
          : t("star", $preferencesStore.language)}
        aria-label={note.is_starred
          ? t("starRemoved", $preferencesStore.language)
          : t("star", $preferencesStore.language)}
        aria-pressed={note.is_starred}
        on:click={toggleStar}
      >
        {note.is_starred ? "★" : "☆"}
      </button>
    </div>
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
        {#if sourceMode}
          <textarea
            bind:this={sourceTextarea}
            class="editor-source-input"
            value={draftContent}
            spellcheck="false"
            aria-label={t("sourceMode", $preferencesStore.language)}
            on:input={(event) => scheduleContentSave(event.currentTarget.value)}
          ></textarea>
        {:else}
          <MarkdownEditor
            value={draftContent}
            onChange={scheduleContentSave}
            bindEditor={bindMarkdownEditor}
          />
        {/if}
      </div>
      {#if detailsOpen && !focusMode}
        <EditorInspector {note} />
      {/if}
    </div>
    {#if shareOpen}
      <ShareImageDialog
        title={draftTitle}
        content={draftContent}
        defaultSignature={$authStore.user?.share_signature ?? "Smarticky"}
        onClose={() => (shareOpen = false)}
      />
    {/if}
  {:else}
    <div class="editor-empty">
      <p class="editor-empty-text">{t("selectOrCreate", $preferencesStore.language)}</p>
    </div>
  {/if}
</section>
