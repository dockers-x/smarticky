<script lang="ts">
  import {
    ChevronLeft,
    ChevronRight,
    CloudUpload,
    CodeXml,
    FileCode,
    Image as ImageIcon,
    LockKeyhole,
    Network,
    PencilRuler,
    Plus,
    Shield,
    Tag as TagIcon,
    X,
  } from "@lucide/svelte";
  import { onDestroy, onMount, tick, type Component } from "svelte";
  import type { Note } from "../../api/types";
  import { createWhiteboard } from "../../api/whiteboards";
  import {
    decryptNoteContent,
    encryptNoteContent,
    type EncryptedNotePayload,
  } from "../../crypto/noteEncryption";
  import { downloadMarkdownFile } from "../../editor/exportMarkdown";
  import type { MarkdownEditorHandle } from "../../editor/markdown";
  import {
    createMermaidDiagramFence,
    mermaidDiagramVariants,
    type MermaidDiagramVariant,
    type MermaidDiagramVariantGroup,
  } from "../../markdown/diagrams/mermaidSyntax";
  import {
    createWhiteboardReferenceFence,
    removeWhiteboardReferenceFences,
    whiteboardRuntime,
  } from "../../markdown/whiteboards";
  import { authStore } from "../../stores/auth";
  import { confirmDialog, notify } from "../../stores/dialogs";
  import { uploadAttachment } from "../../stores/attachments";
  import {
    buildFolderTree,
    flattenFolderTree,
    foldersStore,
  } from "../../stores/folders";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t, type Language } from "../../stores/preferences";
  import { tagsStore } from "../../stores/tags";
  import { whiteboardStore } from "../../stores/whiteboard";
  import EditorInspector from "./EditorInspector.svelte";
  import NoteConnectionPushDialog from "./NoteConnectionPushDialog.svelte";
  import NoteProtectionDialog from "./NoteProtectionDialog.svelte";
  import PasswordField from "../common/PasswordField.svelte";
  import ShareImageDialog from "./ShareImageDialog.svelte";

  export let note: Note | null = null;

  type SaveStatus = "idle" | "saving" | "saved" | "error";
  type MarkdownEditorComponent = Component<{
    value: string;
    noteId: string;
    onChange: (value: string) => void;
    bindEditor: (editor: MarkdownEditorHandle | null) => void;
  }>;

  let markdownEditor: MarkdownEditorHandle | null = null;
  let MarkdownEditor: MarkdownEditorComponent | null = null;
  let activeNoteID = "";
  let draftTitle = "";
  let draftContent = "";
  let imageFileInput: HTMLInputElement | null = null;
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
  let connectionPushOpen = false;
  let protectionOpen = false;
  let protectionBusy = false;
  let protectionError = "";
  let unlockPassword = "";
  let unlockBusy = false;
  let unlockError = "";
  let decryptPassword = "";
  let decryptBusy = false;
  let decryptError = "";
  let decryptedNoteID = "";
  let encryptionSessionPassword = "";
  let actionMenuOpen = false;
  let diagramMenuOpen = false;
  let diagramMenuView: "root" | "mermaid" = "root";
  let folderMenuOpen = false;
  let tagMenuOpen = false;
  let tagName = "";
  let tagBusy = false;
  let tagError = "";
  let unregisterWhiteboardReferenceRemoval: (() => void) | null = null;

  const mermaidGroupOrder: MermaidDiagramVariantGroup[] = [
    "flow",
    "structure",
    "planning",
    "data",
    "advanced",
  ];

  const drawioTemplate = `\`\`\`drawio
<mxfile>
  <diagram name="Page-1">
    <mxGraphModel>
      <root>
        <mxCell id="0"/>
        <mxCell id="1" parent="0"/>
        <mxCell id="2" value="Hello" style="rounded=1;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="80" y="80" width="120" height="60" as="geometry"/>
        </mxCell>
      </root>
    </mxGraphModel>
  </diagram>
</mxfile>
\`\`\``;

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
          timeZone: $preferencesStore.timeZone,
        },
      )
    : "";
  $: visibleNoteTags = note?.tags?.slice(0, 4) ?? [];
  $: hiddenNoteTagCount = Math.max(
    (note?.tags?.length ?? 0) - visibleNoteTags.length,
    0,
  );
  $: noteTagIDs = new Set(note?.tags?.map((tag) => tag.id) ?? []);
  $: availableNoteTags = $tagsStore.filter((tag) => !noteTagIDs.has(tag.id));
  $: wordCount = draftContent.replace(/\s/g, "").length;
  $: isPasswordLocked =
    note?.protection_mode === "password" && note.content_redacted;
  $: isEncryptedNote = note?.protection_mode === "encrypted";
  $: isEncryptedUnlocked =
    Boolean(note) &&
    isEncryptedNote &&
    decryptedNoteID === note?.id &&
    Boolean(encryptionSessionPassword);
  $: isEncryptedLocked = Boolean(note) && isEncryptedNote && !isEncryptedUnlocked;
  $: contentLocked = isPasswordLocked || isEncryptedLocked;
  $: folderOptions = flattenFolderTree(buildFolderTree($foldersStore.folders));
  $: currentFolder = note?.folder_id
    ? $foldersStore.folders.find((folder) => folder.id === note?.folder_id)
    : null;
  $: folderLabel = currentFolder?.name || t("unfiledNotes", $preferencesStore.language);
  $: mermaidVariantGroups = mermaidGroupOrder
    .map((group) => ({
      group,
      label: getMermaidGroupLabel(group, $preferencesStore.language),
      variants: mermaidDiagramVariants.filter((variant) => variant.group === group),
    }))
    .filter((group) => group.variants.length > 0);

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
    draftContent =
      nextNote?.protection_mode === "encrypted" && nextNote.id !== decryptedNoteID
        ? ""
        : (nextNote?.content ?? "");
    saveStatus = nextNote ? "saved" : "idle";
    sourceMode = false;
    detailsOpen = false;
    shareOpen = false;
    connectionPushOpen = false;
    actionMenuOpen = false;
    diagramMenuOpen = false;
    diagramMenuView = "root";
    folderMenuOpen = false;
    tagMenuOpen = false;
    tagName = "";
    tagError = "";
    protectionOpen = false;
    protectionBusy = false;
    protectionError = "";
    unlockPassword = "";
    unlockError = "";
    unlockBusy = false;
    decryptPassword = "";
    decryptError = "";
    decryptBusy = false;
    if (nextNote?.id !== decryptedNoteID) {
      decryptedNoteID = "";
      encryptionSessionPassword = "";
    }
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
    if (contentLocked) return;
    draftContent = value;
    void tick().then(resizeSourceInput);
    clearTimer(contentTimer);
    const noteID = activeNoteID;
    contentTimer = setTimeout(() => {
      void persistDraft(noteID, { content: value });
    }, 500);
  }

  function encryptedPayloadFor(currentNote: Note): EncryptedNotePayload | null {
    if (
      !currentNote.encrypted_content ||
      !currentNote.encryption_alg ||
      !currentNote.encryption_kdf ||
      !currentNote.encryption_salt ||
      !currentNote.encryption_nonce
    ) {
      return null;
    }
    return {
      encrypted_content: currentNote.encrypted_content,
      encryption_alg: currentNote.encryption_alg as EncryptedNotePayload["encryption_alg"],
      encryption_kdf: currentNote.encryption_kdf,
      encryption_salt: currentNote.encryption_salt,
      encryption_nonce: currentNote.encryption_nonce,
    };
  }

  async function persistDraft(
    noteID: string,
    fields: Partial<
      Pick<Note, "title" | "content" | "color" | "is_starred" | "is_deleted" | "folder_id">
    >,
  ): Promise<void> {
    if (!noteID || noteID !== activeNoteID) return;

    const sequence = ++saveSequence;
    saveStatus = "saving";

    try {
      if (note?.protection_mode === "encrypted" && "content" in fields) {
        if (!encryptionSessionPassword || decryptedNoteID !== note.id) {
          throw new Error("Encrypted note is locked.");
        }
        const { content, ...plainFields } = fields;
        const encrypted = await encryptNoteContent(content ?? "", encryptionSessionPassword);
        await notesStore.updateProtection({
          ...plainFields,
          protection_mode: "encrypted",
          ...encrypted,
        });
      } else {
        await notesStore.updateSelected(fields);
      }
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
      Pick<Note, "title" | "content" | "color" | "is_starred" | "is_deleted" | "folder_id">
    > = {};
    if (draftTitle !== note.title) fields.title = draftTitle;
    if (!contentLocked && draftContent !== note.content) fields.content = draftContent;

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

  async function unlockPasswordProtectedNote(): Promise<void> {
    if (!note || unlockBusy) return;
    const password = unlockPassword.trim();
    if (!password) {
      unlockError = t("noteProtectionPasswordRequired", $preferencesStore.language);
      return;
    }

    unlockBusy = true;
    unlockError = "";
    try {
      const unlocked = await notesStore.verifyPassword(note.id, password);
      notesStore.replaceSelected(unlocked);
      draftTitle = unlocked.title;
      draftContent = unlocked.content ?? "";
      unlockPassword = "";
      notify(t("noteUnlocked", $preferencesStore.language), "success");
    } catch {
      unlockError = t("noteUnlockFailed", $preferencesStore.language);
    } finally {
      unlockBusy = false;
    }
  }

  async function decryptEncryptedNote(): Promise<void> {
    if (!note || decryptBusy) return;
    const password = decryptPassword;
    if (!password) {
      decryptError = t("noteProtectionPasswordRequired", $preferencesStore.language);
      return;
    }
    const payload = encryptedPayloadFor(note);
    if (!payload) {
      decryptError = t("noteDecryptFailed", $preferencesStore.language);
      return;
    }

    decryptBusy = true;
    decryptError = "";
    try {
      draftContent = await decryptNoteContent(payload, password);
      decryptedNoteID = note.id;
      encryptionSessionPassword = password;
      decryptPassword = "";
      saveStatus = "saved";
      await tick();
      resizeSourceInput();
      notify(t("noteDecrypted", $preferencesStore.language), "success");
    } catch {
      decryptError = t("noteDecryptFailed", $preferencesStore.language);
    } finally {
      decryptBusy = false;
    }
  }

  async function saveProtection(mode: Note["protection_mode"], password: string): Promise<void> {
    if (!note || protectionBusy) return;
    protectionBusy = true;
    protectionError = "";
    try {
      if ((mode === "none" || mode === "password") && isEncryptedNote && !isEncryptedUnlocked) {
        throw new Error(t("noteDecryptFailed", $preferencesStore.language));
      }

      await flushDraft();

      if (mode === "encrypted") {
        const encrypted = await encryptNoteContent(draftContent, password);
        await notesStore.updateProtection({
          protection_mode: "encrypted",
          ...encrypted,
        });
        decryptedNoteID = note.id;
        encryptionSessionPassword = password;
      } else if (mode === "password") {
        await notesStore.updateProtection({
          protection_mode: "password",
          protection_password: password,
          ...(isEncryptedNote ? { content: draftContent } : {}),
        });
        decryptedNoteID = "";
        encryptionSessionPassword = "";
      } else {
        await notesStore.updateProtection({
          protection_mode: "none",
          ...(isEncryptedNote ? { content: draftContent } : {}),
        });
        decryptedNoteID = "";
        encryptionSessionPassword = "";
      }

      protectionOpen = false;
      notify(t("noteProtectionSaved", $preferencesStore.language), "success");
    } catch (error) {
      protectionError =
        error instanceof Error
          ? error.message
          : t("noteProtectionFailed", $preferencesStore.language);
    } finally {
      protectionBusy = false;
    }
  }

  function exportCurrentNoteMarkdown(): void {
    if (!note || contentLocked) return;
    downloadMarkdownFile(
      draftTitle,
      draftContent,
      t("untitled", $preferencesStore.language),
    );
    notify(t("markdownDownloaded", $preferencesStore.language), "success");
    void flushDraft();
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

  function markdownForAttachmentImage(file: File, downloadURL: string): string {
    const alt = file.name.replace(/\.[^.]+$/, "").trim() || t("imageInsertAlt", $preferencesStore.language);
    return `![${alt}](${downloadURL})`;
  }

  function runImageInsert(): void {
    if (contentLocked) return;
    imageFileInput?.click();
  }

  async function handleImageFileSelected(event: Event): Promise<void> {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    input.value = "";
    if (!file || !note || contentLocked) return;

    try {
      const attachment = await uploadAttachment(note.id, file);
      const markdown = markdownForAttachmentImage(
        file,
        attachment.download_url || `/api/attachments/${attachment.id}/download`,
      );
      insertImageMarkdown(markdown);
    } catch {
      notify(t("uploadAttachmentFailed", $preferencesStore.language), "error");
    }
  }

  function insertImageMarkdown(markdown: string): void {
    if (contentLocked) return;
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

  function withBlockSpacing(markdown: string, before: string, after: string): string {
    const trimmed = markdown.trim();
    const prefix =
      before.length === 0 || before.endsWith("\n\n")
        ? ""
        : before.endsWith("\n")
          ? "\n"
          : "\n\n";
    const suffix =
      after.length === 0 || after.startsWith("\n\n")
        ? "\n\n"
        : after.startsWith("\n")
          ? "\n"
          : "\n\n";

    return `${prefix}${trimmed}${suffix}`;
  }

  function insertBlockIntoSource(markdown: string): void {
    if (!sourceTextarea) {
      const snippet = withBlockSpacing(markdown, draftContent, "");
      scheduleContentSave(`${draftContent}${snippet}`);
      return;
    }

    const start = sourceTextarea.selectionStart ?? draftContent.length;
    const end = sourceTextarea.selectionEnd ?? start;
    const before = draftContent.slice(0, start);
    const after = draftContent.slice(end);
    const snippet = withBlockSpacing(markdown, before, after);
    const nextValue = `${before}${snippet}${after}`;
    scheduleContentSave(nextValue);
    void tick().then(() => {
      const cursor = start + snippet.length;
      sourceTextarea?.focus();
      sourceTextarea?.setSelectionRange(cursor, cursor);
      resizeSourceInput();
    });
  }

  function insertBlockMarkdown(markdown: string): void {
    if (contentLocked) return;
    if (sourceMode) {
      insertBlockIntoSource(markdown);
      return;
    }

    const snippet = withBlockSpacing(markdown, draftContent, "");
    scheduleContentSave(`${draftContent}${snippet}`);
    void tick().then(() => markdownEditor?.focus());
  }

  function insertMermaidTemplate(variant: MermaidDiagramVariant): void {
    closeDiagramMenu();
    insertBlockMarkdown(createMermaidDiagramFence(variant));
  }

  function insertDrawioTemplate(): void {
    closeDiagramMenu();
    insertBlockMarkdown(drawioTemplate);
  }

  async function insertExcalidrawWhiteboard(): Promise<void> {
    if (!note) return;

    closeDiagramMenu();
    try {
      const whiteboard = await createWhiteboard(note.id, {
        title: t("whiteboard", $preferencesStore.language),
        scene_json: "{}",
      });
      insertBlockMarkdown(createWhiteboardReferenceFence(whiteboard.id));
      whiteboardStore.open(whiteboard.id);
    } catch {
      notify(t("whiteboardCreateFailed", $preferencesStore.language), "error");
    }
  }

  function openWhiteboard(whiteboardID: string): void {
    if (!whiteboardID) return;
    actionMenuOpen = false;
    diagramMenuOpen = false;
    whiteboardStore.open(whiteboardID);
  }

  async function removeWhiteboardReferenceFromDraft(
    whiteboardID: string,
    noteID: string,
  ): Promise<{ handled: boolean; removedCount: number }> {
    if (!note || note.id !== noteID || activeNoteID !== noteID) {
      return { handled: false, removedCount: 0 };
    }

    const removal = removeWhiteboardReferenceFences(draftContent, whiteboardID);
    if (removal.removedCount === 0) {
      return { handled: true, removedCount: 0 };
    }

    clearTimer(contentTimer);
    contentTimer = null;
    draftContent = removal.markdown;
    await tick();
    resizeSourceInput();
    await persistDraft(noteID, { content: removal.markdown });
    return { handled: true, removedCount: removal.removedCount };
  }

  function getMermaidGroupLabel(
    group: MermaidDiagramVariantGroup,
    language: Language,
  ): string {
    switch (group) {
      case "flow":
        return t("mermaidGroupFlow", language);
      case "structure":
        return t("mermaidGroupStructure", language);
      case "planning":
        return t("mermaidGroupPlanning", language);
      case "data":
        return t("mermaidGroupData", language);
      case "advanced":
        return t("mermaidGroupAdvanced", language);
    }
  }

  function toggleSourceMode(): void {
    if (contentLocked) return;
    sourceMode = !sourceMode;
    actionMenuOpen = false;
    diagramMenuOpen = false;
    diagramMenuView = "root";
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
    if (!note || shareOpen || contentLocked) return;
    if (!(event.metaKey || event.ctrlKey) || event.shiftKey) return;
    if (event.key !== "/" && event.code !== "Slash") return;

    event.preventDefault();
    toggleSourceMode();
  }

  async function moveCurrentNoteToFolder(folderID: string | null): Promise<void> {
    if (!note) return;
    try {
      await notesStore.moveToFolder([note.id], folderID);
      await Promise.all([notesStore.load(), foldersStore.load()]);
      const refreshed = await notesStore.getByID(note.id);
      notesStore.replaceSelected(refreshed);
      folderMenuOpen = false;
      notify(t("movedNotes", $preferencesStore.language), "success");
    } catch {
      notify(t("moveNotesFailed", $preferencesStore.language), "error");
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

  function closeDiagramMenu(): void {
    diagramMenuOpen = false;
    diagramMenuView = "root";
  }

  function toggleDiagramMenu(): void {
    diagramMenuOpen = !diagramMenuOpen;
    if (diagramMenuOpen) {
      actionMenuOpen = false;
      folderMenuOpen = false;
      tagMenuOpen = false;
      diagramMenuView = "root";
      return;
    }
    diagramMenuView = "root";
  }

  async function refreshSelectedNote(): Promise<void> {
    if (!note) return;
    await notesStore.load();
    const refreshed = await notesStore.getByID(note.id);
    notesStore.replaceSelected(refreshed);
  }

  async function addTagToCurrentNote(name: string): Promise<void> {
    if (!note) return;
    const trimmed = name.trim();
    if (!trimmed || tagBusy) return;
    if (note.tags?.some((tag) => tag.name.toLowerCase() === trimmed.toLowerCase())) {
      tagName = "";
      return;
    }

    tagBusy = true;
    tagError = "";
    try {
      await tagsStore.addToNote(note.id, trimmed);
      await refreshSelectedNote();
      tagName = "";
    } catch (error) {
      tagError =
        error instanceof Error
          ? error.message
          : t("addTagFailed", $preferencesStore.language);
    } finally {
      tagBusy = false;
    }
  }

  async function removeTagFromCurrentNote(tagID: string): Promise<void> {
    if (!note || tagBusy) return;

    tagBusy = true;
    tagError = "";
    try {
      await tagsStore.removeFromNote(note.id, tagID);
      await refreshSelectedNote();
    } catch (error) {
      tagError =
        error instanceof Error
          ? error.message
          : t("addTagFailed", $preferencesStore.language);
    } finally {
      tagBusy = false;
    }
  }

  async function runMenuAction(action: () => void | Promise<void>): Promise<void> {
    closeActionMenu();
    await action();
  }

  onMount(() => {
    void foldersStore.load();
    void tagsStore.load();
    void import("./MarkdownEditor.svelte").then((module) => {
      MarkdownEditor = module.default as MarkdownEditorComponent;
    });
    unregisterWhiteboardReferenceRemoval =
      whiteboardStore.registerReferenceRemoval(removeWhiteboardReferenceFromDraft);
  });

  onDestroy(() => {
    clearTimer(titleTimer);
    clearTimer(contentTimer);
    unregisterWhiteboardReferenceRemoval?.();
    unregisterWhiteboardReferenceRemoval = null;
    markdownEditor = null;
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
          disabled={contentLocked || (!markdownEditor && !sourceMode)}
          on:click={runImageInsert}
        >
          <ImageIcon aria-hidden="true" size={19} strokeWidth={2} />
        </button>
        <input
          bind:this={imageFileInput}
          class="visually-hidden"
          type="file"
          accept="image/*"
          tabindex="-1"
          aria-hidden="true"
          on:change={(event) => void handleImageFileSelected(event)}
        />
        <div class="editor-diagram-menu">
          <button
            class="editor-icon-button"
            type="button"
            title={t("insertDiagram", $preferencesStore.language)}
            aria-label={t("insertDiagram", $preferencesStore.language)}
            aria-expanded={diagramMenuOpen}
            disabled={contentLocked || (!markdownEditor && !sourceMode)}
            on:click={toggleDiagramMenu}
          >
            <Network aria-hidden="true" size={19} strokeWidth={2} />
          </button>
          {#if diagramMenuOpen}
            <div class="editor-popover editor-diagram-popover" role="menu">
              {#if diagramMenuView === "root"}
                <p class="editor-popover-title">
                  {t("insertDiagram", $preferencesStore.language)}
                </p>
                <button
                  class="editor-popover-row editor-diagram-row"
                  type="button"
                  role="menuitem"
                  on:click={() => (diagramMenuView = "mermaid")}
                >
                  <span class="editor-diagram-row__text">
                    <span class="editor-diagram-row__title">Mermaid</span>
                    <span class="editor-diagram-row__description">
                      {t("mermaidTemplatesHint", $preferencesStore.language)}
                    </span>
                  </span>
                  <ChevronRight aria-hidden="true" size={17} strokeWidth={2} />
                </button>
                <button
                  class="editor-popover-row editor-diagram-row"
                  type="button"
                  role="menuitem"
                  on:click={insertDrawioTemplate}
                >
                  <CodeXml aria-hidden="true" size={17} strokeWidth={2} />
                  <span class="editor-diagram-row__text">
                    <span class="editor-diagram-row__title">drawio</span>
                    <span class="editor-diagram-row__description">
                      {t("drawioTemplateHint", $preferencesStore.language)}
                    </span>
                  </span>
                </button>
                <button
                  class="editor-popover-row editor-diagram-row"
                  type="button"
                  role="menuitem"
                  on:click={() => void insertExcalidrawWhiteboard()}
                >
                  <PencilRuler aria-hidden="true" size={17} strokeWidth={2} />
                  <span class="editor-diagram-row__text">
                    <span class="editor-diagram-row__title">Excalidraw</span>
                    <span class="editor-diagram-row__description">
                      {t("whiteboardTemplateHint", $preferencesStore.language)}
                    </span>
                  </span>
                </button>
              {:else}
                <button
                  class="editor-popover-row editor-diagram-back"
                  type="button"
                  role="menuitem"
                  on:click={() => (diagramMenuView = "root")}
                >
                  <ChevronLeft aria-hidden="true" size={17} strokeWidth={2} />
                  <span>{t("mermaidTemplates", $preferencesStore.language)}</span>
                </button>
                {#each mermaidVariantGroups as group}
                  <p class="editor-popover-label">{group.label}</p>
                  {#each group.variants as variant (variant.name)}
                    <button
                      class="editor-popover-row editor-diagram-template-row"
                      type="button"
                      role="menuitem"
                      on:click={() => insertMermaidTemplate(variant)}
                    >
                      <span class="editor-diagram-row__text">
                        <span class="editor-diagram-row__title">{variant.label}</span>
                        <span class="editor-diagram-row__description">
                          {variant.description[$preferencesStore.language]}
                        </span>
                      </span>
                      <code>{variant.name}</code>
                    </button>
                  {/each}
                {/each}
              {/if}
            </div>
          {/if}
        </div>
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
          disabled={contentLocked}
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
          <button
            class="editor-share-button"
            type="button"
            disabled={contentLocked}
            on:click={() => (shareOpen = true)}
          >
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
              <button
                class="editor-action-button"
                type="button"
                on:click={() => void runMenuAction(exportCurrentNoteMarkdown)}
                disabled={contentLocked}
              >
                {t("exportMarkdown", $preferencesStore.language)}
              </button>
              <button
                class="editor-action-button"
                type="button"
                disabled={contentLocked}
                on:click={() =>
                  void runMenuAction(() => {
                    connectionPushOpen = true;
                  })}
              >
                <CloudUpload aria-hidden="true" size={15} strokeWidth={2} />
                {t("noteConnectionPush", $preferencesStore.language)}
              </button>
              <button
                class="editor-action-button"
                type="button"
                on:click={() =>
                  void runMenuAction(() => {
                    protectionError = "";
                    protectionOpen = true;
                  })}
              >
                {t("noteProtection", $preferencesStore.language)}
              </button>
              {#if note.is_deleted}
                <button
                  class="editor-action-button"
                  type="button"
                  disabled={contentLocked}
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
                  disabled={contentLocked}
                  on:click={() =>
                    void runMenuAction(() => {
                      shareOpen = true;
                    })}
                >
                  {t("generateImage", $preferencesStore.language)}
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
          class="editor-meta-button editor-folder-button"
          type="button"
          aria-label={`${t("noteLocation", $preferencesStore.language)}: ${folderLabel}`}
          aria-expanded={folderMenuOpen}
          on:click={() => {
            folderMenuOpen = !folderMenuOpen;
            tagMenuOpen = false;
          }}
        >
          <span>{folderLabel}</span>
          <span aria-hidden="true">⌄</span>
        </button>
        {#if folderMenuOpen}
          <div class="editor-popover editor-folder-popover">
            <p class="editor-popover-title">{t("moveToNotebookGroup", $preferencesStore.language)}</p>
            <button
              class:active={!note.folder_id}
              class="editor-popover-row"
              type="button"
              on:click={() => void moveCurrentNoteToFolder(null)}
            >
              <span>{t("unfiledNotes", $preferencesStore.language)}</span>
              {#if !note.folder_id}
                <span aria-hidden="true">✓</span>
              {/if}
            </button>
            {#if folderOptions.length}
              <p class="editor-popover-label">{t("notebookGroups", $preferencesStore.language)}</p>
              {#each folderOptions as option (option.id)}
                <button
                  class:active={note.folder_id === option.id}
                  class="editor-popover-row editor-folder-option"
                  type="button"
                  style={`--folder-depth: ${option.depth - 1}`}
                  on:click={() => void moveCurrentNoteToFolder(option.id)}
                >
                  <span>{option.name}</span>
                  {#if note.folder_id === option.id}
                    <span aria-hidden="true">✓</span>
                  {/if}
                </button>
              {/each}
            {:else}
              <p class="editor-popover-label">{t("folderEmptyTitle", $preferencesStore.language)}</p>
            {/if}
          </div>
        {/if}
      </div>
      <time class="editor-meta-text" datetime={note.updated_at}>{noteDate}</time>
      <span class="editor-meta-text">{wordCount} {t("wordUnit", $preferencesStore.language)}</span>
      <div class="editor-meta-popover-anchor editor-tag-anchor">
        <button
          class="editor-meta-button editor-tags-button"
          type="button"
          aria-label={t("tags", $preferencesStore.language)}
          aria-expanded={tagMenuOpen}
          on:click={() => {
            tagMenuOpen = !tagMenuOpen;
            folderMenuOpen = false;
          }}
        >
          <TagIcon size={14} strokeWidth={1.8} aria-hidden="true" />
          <span>
            {visibleNoteTags.length
              ? `${visibleNoteTags[0].name}${hiddenNoteTagCount > 0 ? ` +${hiddenNoteTagCount}` : ""}`
              : t("tags", $preferencesStore.language)}
          </span>
        </button>
        {#if tagMenuOpen}
          <div class="editor-popover editor-tag-popover">
            <p class="editor-popover-title">{t("tags", $preferencesStore.language)}</p>
            {#if note.tags?.length}
              <div class="editor-tag-chip-list" aria-label={t("currentTags", $preferencesStore.language)}>
                {#each note.tags as tag (tag.id)}
                  <button
                    type="button"
                    disabled={tagBusy}
                    title={tag.name}
                    on:click={() => void removeTagFromCurrentNote(tag.id)}
                  >
                    <span>{tag.name}</span>
                    <X size={12} strokeWidth={2} aria-hidden="true" />
                  </button>
                {/each}
              </div>
            {:else}
              <p class="editor-popover-label">{t("noTags", $preferencesStore.language)}</p>
            {/if}
            {#if availableNoteTags.length}
              <p class="editor-popover-label">{t("availableTags", $preferencesStore.language)}</p>
              <div class="editor-tag-options">
                {#each availableNoteTags as tag (tag.id)}
                  <button
                    type="button"
                    disabled={tagBusy}
                    title={tag.name}
                    on:click={() => void addTagToCurrentNote(tag.name)}
                  >
                    {tag.name}
                  </button>
                {/each}
              </div>
            {/if}
            <form
              class="editor-tag-add-row"
              on:submit|preventDefault={() => void addTagToCurrentNote(tagName)}
            >
              <input
                bind:value={tagName}
                type="text"
                placeholder={t("addTag", $preferencesStore.language)}
                aria-label={t("addTag", $preferencesStore.language)}
                disabled={tagBusy}
              />
              <button
                type="submit"
                aria-label={t("addTag", $preferencesStore.language)}
                disabled={tagBusy || !tagName.trim()}
              >
                <Plus size={14} strokeWidth={2} aria-hidden="true" />
              </button>
            </form>
            {#if tagError}
              <p class="editor-tag-error" role="alert">{tagError}</p>
            {/if}
          </div>
        {/if}
      </div>
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
      <div
        class="editor-surface"
        use:whiteboardRuntime={{ contentKey: draftContent, onOpen: openWhiteboard }}
      >
        <textarea
          bind:this={titleInput}
          class="editor-title-input"
          value={draftTitle}
          placeholder={t("untitled", $preferencesStore.language)}
          aria-label={t("noteTitle", $preferencesStore.language)}
          rows="1"
          on:input={(event) => scheduleTitleSave(event.currentTarget.value)}
        ></textarea>
        {#if isPasswordLocked}
          <div class="editor-lock-state">
            <LockKeyhole size={34} strokeWidth={1.8} aria-hidden="true" />
            <h2>{t("noteLocked", $preferencesStore.language)}</h2>
            <p>{t("noteUnlockHint", $preferencesStore.language)}</p>
            <form on:submit|preventDefault={() => void unlockPasswordProtectedNote()}>
              <PasswordField
                bind:value={unlockPassword}
                label={t("noteAccessPassword", $preferencesStore.language)}
                placeholder={t("noteProtectionPasswordPlaceholder", $preferencesStore.language)}
                error={unlockError}
                disabled={unlockBusy}
                autocomplete="current-password"
                showPasswordLabel={t("showPassword", $preferencesStore.language)}
                hidePasswordLabel={t("hidePassword", $preferencesStore.language)}
              />
              <button type="submit" disabled={unlockBusy}>
                {unlockBusy ? t("loading", $preferencesStore.language) : t("noteUnlock", $preferencesStore.language)}
              </button>
            </form>
          </div>
        {:else if isEncryptedLocked}
          <div class="editor-lock-state">
            <Shield size={34} strokeWidth={1.8} aria-hidden="true" />
            <h2>{t("noteEncryptedLocked", $preferencesStore.language)}</h2>
            <p>{t("noteDecryptHint", $preferencesStore.language)}</p>
            <form on:submit|preventDefault={() => void decryptEncryptedNote()}>
              <PasswordField
                bind:value={decryptPassword}
                label={t("noteEncryptionPassword", $preferencesStore.language)}
                placeholder={t("noteDecryptPasswordPlaceholder", $preferencesStore.language)}
                error={decryptError}
                disabled={decryptBusy}
                autocomplete="current-password"
                showPasswordLabel={t("showPassword", $preferencesStore.language)}
                hidePasswordLabel={t("hidePassword", $preferencesStore.language)}
              />
              <button type="submit" disabled={decryptBusy}>
                {decryptBusy ? t("loading", $preferencesStore.language) : t("noteDecrypt", $preferencesStore.language)}
              </button>
            </form>
          </div>
        {:else if sourceMode}
          <textarea
            bind:this={sourceTextarea}
            class="editor-source-input"
            value={draftContent}
            spellcheck="false"
            aria-label={t("sourceMode", $preferencesStore.language)}
            on:input={(event) => scheduleContentSave(event.currentTarget.value)}
          ></textarea>
        {:else}
          {#if MarkdownEditor}
            <svelte:component
              this={MarkdownEditor}
              value={draftContent}
              noteId={note.id}
              onChange={scheduleContentSave}
              bindEditor={bindMarkdownEditor}
            />
          {:else}
            <div class="markdown-editor-host markdown-editor-host--loading"></div>
          {/if}
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
    {#if connectionPushOpen}
      <NoteConnectionPushDialog
        {note}
        onClose={() => (connectionPushOpen = false)}
      />
    {/if}
    {#if protectionOpen}
      <NoteProtectionDialog
        currentMode={note.protection_mode}
        busy={protectionBusy}
        error={protectionError}
        onClose={() => {
          if (!protectionBusy) protectionOpen = false;
        }}
        onSave={saveProtection}
      />
    {/if}
  {:else}
    <div class="editor-empty">
      <p class="editor-empty-text">{t("selectOrCreate", $preferencesStore.language)}</p>
    </div>
  {/if}
</section>
