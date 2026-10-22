<script lang="ts">
  import {
    ChevronLeft,
    ChevronRight,
    Check,
    CloudUpload,
    CodeXml,
    Download,
    Ellipsis,
    FileCode,
    FilePenLine,
    Focus,
    Image as ImageIcon,
    Info,
    LockKeyhole,
    Network,
    PencilRuler,
    Plus,
    RotateCcw,
    RotateCw,
    Shield,
    Star,
    Tag as TagIcon,
    Trash2,
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
  import { captureEditorMarkdown, type MarkdownEditorHandle } from "../../editor/markdown";
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
  import {
    createEditorDraft,
    type EditorDraft,
    type EditorDraftFields,
    type EditorSaveStatus,
  } from "./draftSave";
  import NoteConnectionPushDialog from "./NoteConnectionPushDialog.svelte";
  import NoteProtectionDialog from "./NoteProtectionDialog.svelte";
  import PasswordField from "../common/PasswordField.svelte";
  import ShareImageDialog from "./ShareImageDialog.svelte";

  export let note: Note | null = null;

  type DraftSession = {
    noteID: string;
    protectionMode: Note["protection_mode"];
    password: string;
    draft: EditorDraft;
  };
  type EditorMenu = "actions" | "diagram" | "folder" | "tags";
  type MarkdownEditorComponent = Component<{
    value: string;
    noteId: string;
    onChange: (value: string, noteID: string) => void;
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
  let draftTimer: ReturnType<typeof setTimeout> | null = null;
  let draftSession: DraftSession | null = null;
  const retainedDrafts = new Map<string, DraftSession>();
  let saveStatus: EditorSaveStatus = "idle";
  let finishing = false;
  let imageBusy = false;
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
  let actionMenuTrigger: HTMLButtonElement | null = null;
  let diagramMenuTrigger: HTMLButtonElement | null = null;
  let folderMenuTrigger: HTMLButtonElement | null = null;
  let tagMenuTrigger: HTMLButtonElement | null = null;
  let diagramPopover: HTMLDivElement | null = null;
  let menuKeyboardOpen = false;
  let menuFocusLast = false;
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
    pending: $preferencesStore.language === "zh" ? "未保存" : "Unsaved",
    saving: t("saving", $preferencesStore.language),
    saved: t("saved", $preferencesStore.language),
    error: t("saveError", $preferencesStore.language),
  } satisfies Record<EditorSaveStatus, string>;

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
  $: visibleNoteTags = note?.tags?.slice(0, 1) ?? [];
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
    captureCurrentEditor();
    clearTimer(draftTimer);
    draftTimer = null;
    if (draftSession?.draft.dirty) {
      const previous = draftSession;
      retainedDrafts.set(previous.noteID, previous);
      void previous.draft.flush().then((saved) => {
        if (!saved) {
          notify(`${t("saveError", $preferencesStore.language)}: ${previous.draft.value.title || t("untitled", $preferencesStore.language)}`, "error");
        }
      });
    }
    activeNoteID = nextNote?.id ?? "";
    draftSession = nextNote
      ? retainedDrafts.get(nextNote.id) ?? createDraftSession(nextNote)
      : null;
    draftTitle = draftSession?.draft.value.title ?? "";
    draftContent =
      nextNote?.protection_mode === "encrypted" && nextNote.id !== decryptedNoteID
        ? ""
        : (draftSession?.draft.value.content ?? "");
    saveStatus = draftSession?.draft.status ?? "idle";
    finishing = false;
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

  function createDraftSession(currentNote: Note): DraftSession {
    const session: DraftSession = {
      noteID: currentNote.id,
      protectionMode: currentNote.protection_mode,
      password: "",
      draft: createEditorDraft(
        {
          title: currentNote.title,
          content: currentNote.protection_mode === "encrypted" ? "" : currentNote.content ?? "",
        },
        async (fields) => {
          if (session.protectionMode === "encrypted" && "content" in fields) {
            if (!session.password) throw new Error("Encrypted note is locked.");
            const { content, ...plainFields } = fields;
            const encrypted = await encryptNoteContent(content ?? "", session.password);
            await notesStore.updateNote(session.noteID, {
              ...plainFields,
              protection_mode: "encrypted",
              ...encrypted,
            });
          } else {
            await notesStore.updateNote(session.noteID, fields);
          }
        },
        (status) => {
          if (status === "saved") retainedDrafts.delete(session.noteID);
          if (draftSession === session) saveStatus = status;
        },
      ),
    };
    return session;
  }

  $: if ((note?.id ?? "") !== activeNoteID) {
    resetDraft(note);
  }

  function bindMarkdownEditor(editor: MarkdownEditorHandle | null): void {
    markdownEditor = editor;
  }

  function captureCurrentEditor(): void {
    if (sourceMode || !draftSession) return;
    const markdown = captureEditorMarkdown(markdownEditor, draftSession.noteID);
    if (markdown === null) return;
    draftContent = markdown;
    draftSession.draft.update({ content: markdown });
  }

  function scheduleTitleSave(value: string): void {
    draftTitle = value;
    resizeTitleInput();
    draftSession?.draft.update({ title: value });
    scheduleDraftSave();
  }

  function scheduleDraftSave(): void {
    clearTimer(draftTimer);
    draftTimer = setTimeout(() => {
      draftTimer = null;
      void draftSession?.draft.flush();
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

  function scheduleContentSave(value: string, noteID = activeNoteID): void {
    if (contentLocked || noteID !== draftSession?.noteID) return;
    draftContent = value;
    void tick().then(resizeSourceInput);
    draftSession?.draft.update({ content: value });
    scheduleDraftSave();
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
    fields: EditorDraftFields,
  ): Promise<boolean> {
    if (!draftSession || noteID !== draftSession.noteID) return false;
    draftSession.draft.update(fields);
    return flushDraft(false);
  }

  async function flushDraft(captureLiveEditor = true): Promise<boolean> {
    if (captureLiveEditor) captureCurrentEditor();
    clearTimer(draftTimer);
    draftTimer = null;
    return draftSession ? draftSession.draft.flush() : true;
  }

  async function finishEditing(): Promise<void> {
    if (finishing) return;
    finishing = true;
    const session = draftSession;
    try {
      if (!(await flushDraft())) {
        notify(t("saveError", $preferencesStore.language), "error");
        return;
      }
      if (draftSession !== session) return;
      notesStore.clearSelection();
    } finally {
      finishing = false;
    }
  }

  async function unlockPasswordProtectedNote(): Promise<void> {
    if (!note || unlockBusy) return;
    const noteID = note.id;
    const session = draftSession;
    const password = unlockPassword.trim();
    if (!password) {
      unlockError = t("noteProtectionPasswordRequired", $preferencesStore.language);
      return;
    }

    unlockBusy = true;
    unlockError = "";
    try {
      const unlocked = await notesStore.verifyPassword(noteID, password);
      if (draftSession !== session) return;
      notesStore.replaceSelected(unlocked);
      if (session && !("content" in session.draft.changes)) {
        session.draft.accept({ content: unlocked.content ?? "" });
      }
      draftContent = session?.draft.value.content ?? "";
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
    const noteID = note.id;
    const session = draftSession;
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
      const plaintext = await decryptNoteContent(payload, password);
      if (!session || draftSession !== session) return;
      if (!("content" in session.draft.changes)) session.draft.accept({ content: plaintext });
      draftContent = session.draft.value.content;
      session.password = password;
      decryptedNoteID = noteID;
      encryptionSessionPassword = password;
      decryptPassword = "";
      saveStatus = session.draft.status;
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
    const noteID = note.id;
    const session = draftSession;
    protectionBusy = true;
    protectionError = "";
    try {
      if ((mode === "none" || mode === "password") && isEncryptedNote && !isEncryptedUnlocked) {
        throw new Error(t("noteDecryptFailed", $preferencesStore.language));
      }

      if (!(await flushDraft())) throw new Error(t("saveError", $preferencesStore.language));
      if (draftSession !== session) return;

      if (mode === "encrypted") {
        const encrypted = await encryptNoteContent(draftContent, password);
        await notesStore.updateNote(noteID, {
          protection_mode: "encrypted",
          ...encrypted,
        });
        decryptedNoteID = noteID;
        encryptionSessionPassword = password;
      } else if (mode === "password") {
        await notesStore.updateNote(noteID, {
          protection_mode: "password",
          protection_password: password,
          ...(isEncryptedNote ? { content: draftContent } : {}),
        });
        decryptedNoteID = "";
        encryptionSessionPassword = "";
      } else {
        await notesStore.updateNote(noteID, {
          protection_mode: "none",
          ...(isEncryptedNote ? { content: draftContent } : {}),
        });
        decryptedNoteID = "";
        encryptionSessionPassword = "";
      }

      if (session) {
        session.protectionMode = mode;
        session.password = mode === "encrypted" ? password : "";
        session.draft.accept({ content: draftContent });
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
    captureCurrentEditor();
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
    const noteID = note.id;
    const wasStarred = note.is_starred;

    try {
      await notesStore.updateNote(noteID, { is_starred: !wasStarred });
      notify(
        wasStarred
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
    if (contentLocked || imageBusy) return;
    closeDiagramMenu();
    imageFileInput?.click();
  }

  async function handleImageFileSelected(event: Event): Promise<void> {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    input.value = "";
    if (!file || !note || contentLocked || imageBusy) return;
    const noteID = note.id;
    imageBusy = true;

    try {
      const attachment = await uploadAttachment(noteID, file);
      if (activeNoteID !== noteID) return;
      const markdown = markdownForAttachmentImage(
        file,
        attachment.download_url || `/api/attachments/${attachment.id}/download`,
      );
      insertImageMarkdown(markdown);
    } catch {
      notify(t("uploadAttachmentFailed", $preferencesStore.language), "error");
    } finally {
      imageBusy = false;
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

    draftContent = removal.markdown;
    await tick();
    resizeSourceInput();
    if (!(await persistDraft(noteID, { content: removal.markdown }))) {
      throw new Error(t("saveError", $preferencesStore.language));
    }
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
    captureCurrentEditor();
    if (window.matchMedia("(max-width: 960px)").matches) detailsOpen = false;
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
    if (!note || shareOpen || protectionOpen || connectionPushOpen) return;
    if (!(event.metaKey || event.ctrlKey) || event.shiftKey) return;
    if (event.key.toLowerCase() === "s") {
      event.preventDefault();
      void flushDraft();
      return;
    }
    if (contentLocked) return;
    if (event.key !== "/" && event.code !== "Slash") return;

    event.preventDefault();
    toggleSourceMode();
  }

  function handleBeforeUnload(event: BeforeUnloadEvent): void {
    captureCurrentEditor();
    if (!draftSession?.draft.dirty && ![...retainedDrafts.values()].some((session) => session.draft.dirty)) return;
    event.preventDefault();
    event.returnValue = "";
  }

  async function moveCurrentNoteToFolder(folderID: string | null): Promise<void> {
    if (!note) return;
    const noteID = note.id;
    try {
      await notesStore.moveToFolder([noteID], folderID);
      await Promise.all([
        notesStore.load(),
        notesStore.loadCalendarNotes(),
        foldersStore.load(),
      ]);
      const refreshed = await notesStore.getByID(noteID);
      if (activeNoteID !== noteID) return;
      notesStore.replaceSelected(refreshed);
      folderMenuOpen = false;
      notify(t("movedNotes", $preferencesStore.language), "success");
    } catch {
      notify(t("moveNotesFailed", $preferencesStore.language), "error");
    }
  }

  async function toggleTrash(): Promise<void> {
    if (!note) return;
    const noteID = note.id;

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
      if (!(await flushDraft())) return;
      await notesStore.updateNote(noteID, { is_deleted: !restoring });
      if (activeNoteID === noteID) notesStore.clearSelection();
      await Promise.all([notesStore.load(), notesStore.loadCalendarNotes()]);
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
    const noteID = note.id;

    const confirmed = await confirmDialog({
      title: t("deleteForever", $preferencesStore.language),
      message: t("deleteForeverMessage", $preferencesStore.language),
      confirmLabel: t("deleteForever", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
    });
    if (!confirmed) return;

    try {
      if (!(await flushDraft())) return;
      await notesStore.deletePermanent(noteID);
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

  function toggleMenu(menu: EditorMenu, event: MouseEvent | KeyboardEvent): void {
    const wasOpen = {
      actions: actionMenuOpen,
      diagram: diagramMenuOpen,
      folder: folderMenuOpen,
      tags: tagMenuOpen,
    }[menu];
    actionMenuOpen = !wasOpen && menu === "actions";
    diagramMenuOpen = !wasOpen && menu === "diagram";
    folderMenuOpen = !wasOpen && menu === "folder";
    tagMenuOpen = !wasOpen && menu === "tags";
    menuKeyboardOpen = event.type === "keydown" || event.detail === 0;
    menuFocusLast = event instanceof KeyboardEvent && event.key === "ArrowUp";
    diagramMenuView = "root";
  }

  function openMenuWithKeyboard(menu: EditorMenu, event: KeyboardEvent): void {
    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
    event.preventDefault();
    const trigger = event.currentTarget as HTMLButtonElement;
    if (trigger.getAttribute("aria-expanded") === "true") {
      const panel = document.getElementById(trigger.getAttribute("aria-controls") ?? "");
      const items = Array.from(panel?.querySelectorAll<HTMLButtonElement>("button:not(:disabled)") ?? []);
      items[event.key === "ArrowUp" ? items.length - 1 : 0]?.focus();
      return;
    }
    toggleMenu(menu, event);
  }

  function editorPopover(
    node: HTMLElement,
    options: { close: () => void; trigger: HTMLButtonElement | null },
  ) {
    const { close, trigger } = options;
    const focusable = () => Array.from(node.querySelectorAll<HTMLElement>(
      'button:not(:disabled), input:not(:disabled), [href], [tabindex="0"]',
    ));
    if (menuKeyboardOpen) void tick().then(() => {
      const items = focusable();
      items[menuFocusLast ? items.length - 1 : 0]?.focus();
    });

    function outside(event: Event): void {
      const target = event.target as Node;
      if (!node.contains(target) && !trigger?.contains(target)) close();
    }

    function keydown(event: KeyboardEvent): void {
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        close();
        trigger?.focus();
        return;
      }
      if (node.getAttribute("role") !== "menu" || !node.contains(event.target as Node)) return;
      if (!["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) return;
      const items = focusable();
      const current = items.indexOf(document.activeElement as HTMLElement);
      const next = event.key === "Home" ? 0
        : event.key === "End" ? items.length - 1
          : (current + (event.key === "ArrowDown" ? 1 : -1) + items.length) % items.length;
      event.preventDefault();
      items[next]?.focus();
    }

    document.addEventListener("pointerdown", outside);
    document.addEventListener("focusin", outside);
    document.addEventListener("keydown", keydown);
    return {
      destroy() {
        document.removeEventListener("pointerdown", outside);
        document.removeEventListener("focusin", outside);
        document.removeEventListener("keydown", keydown);
      },
    };
  }

  async function showDiagramTemplates(view: "root" | "mermaid"): Promise<void> {
    diagramMenuView = view;
    await tick();
    diagramPopover?.querySelector<HTMLButtonElement>("button")?.focus();
  }

  async function refreshSelectedNote(): Promise<void> {
    if (!note) return;
    const noteID = note.id;
    await Promise.all([notesStore.load(), notesStore.loadCalendarNotes()]);
    const refreshed = await notesStore.getByID(noteID);
    if (activeNoteID === noteID) notesStore.replaceSelected(refreshed);
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
    captureCurrentEditor();
    closeActionMenu();
    actionMenuTrigger?.focus();
    await action();
  }

  onMount(() => {
    void foldersStore.load();
    void tagsStore.load();
    void import("./MarkdownEditor.svelte")
      .then((module) => {
        MarkdownEditor = module.default as MarkdownEditorComponent;
      })
      .catch(() => {
        sourceMode = true;
        void tick().then(resizeSourceInput);
        notify($preferencesStore.language === "zh" ? "编辑器加载失败，已切换为 Markdown 源码。" : "The editor could not load. Markdown source is available instead.", "error");
      });
    unregisterWhiteboardReferenceRemoval =
      whiteboardStore.registerReferenceRemoval(removeWhiteboardReferenceFromDraft);
  });

  onDestroy(() => {
    captureCurrentEditor();
    clearTimer(draftTimer);
    void draftSession?.draft.flush();
    unregisterWhiteboardReferenceRemoval?.();
    unregisterWhiteboardReferenceRemoval = null;
    markdownEditor = null;
  });
</script>

<svelte:window on:keydown={handleEditorKeydown} on:beforeunload={handleBeforeUnload} />

<section
  class:focus-mode={focusMode}
  class:showing-details={detailsOpen && !focusMode}
  class:has-note={Boolean(note)}
  class="editor-pane"
  aria-label={t("editor", $preferencesStore.language)}
>
  {#if note}
    <header class="editor-header">
      <div class="editor-header__context">
        <button
          class="editor-mobile-back"
          type="button"
          aria-label={t("back", $preferencesStore.language)}
          disabled={finishing}
          on:click={() => void finishEditing()}
        >
          <ChevronLeft aria-hidden="true" size={20} strokeWidth={1.8} />
        </button>
        <div class="editor-save-feedback" role="status" aria-live="polite" aria-atomic="true">
          {#if saveStatus === "error"}
            <button
              class="editor-save-retry"
              type="button"
              aria-label={$preferencesStore.language === "zh" ? "保存失败，点击重试" : "Save failed. Retry saving"}
              on:click={() => void flushDraft()}
            >
              <RotateCw size={13} strokeWidth={1.8} aria-hidden="true" />
              {statusText[saveStatus]}
            </button>
          {:else}
            <span class:visible={saveStatus !== "idle"} class="editor-save-status" data-state={saveStatus}>
              {#if saveStatus === "saved"}<Check size={13} strokeWidth={1.8} aria-hidden="true" />{/if}
              {statusText[saveStatus]}
            </span>
          {/if}
        </div>
      </div>
      <div class="editor-header__right">
        <div class="editor-writing-tools" role="group" aria-label={$preferencesStore.language === "zh" ? "编辑工具" : "Writing tools"}>
        <button
          class="editor-icon-button editor-image-button"
          type="button"
          title={t("insertImage", $preferencesStore.language)}
          aria-label={t("insertImage", $preferencesStore.language)}
          disabled={contentLocked || imageBusy || (!markdownEditor && !sourceMode)}
          aria-busy={imageBusy}
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
            bind:this={diagramMenuTrigger}
            class="editor-icon-button"
            type="button"
            title={$preferencesStore.language === "zh" ? "插入图片或图表" : "Insert image or diagram"}
            aria-label={$preferencesStore.language === "zh" ? "插入图片或图表" : "Insert image or diagram"}
            aria-haspopup="menu"
            aria-controls={`editor-insert-${note.id}`}
            aria-expanded={diagramMenuOpen}
            disabled={contentLocked || (!markdownEditor && !sourceMode)}
            on:click={(event) => toggleMenu("diagram", event)}
            on:keydown={(event) => openMenuWithKeyboard("diagram", event)}
          >
            <Plus aria-hidden="true" size={20} strokeWidth={1.8} />
          </button>
          {#if diagramMenuOpen}
            <div
              bind:this={diagramPopover}
              id={`editor-insert-${note.id}`}
              class="editor-popover editor-diagram-popover"
              role="menu"
              aria-label={$preferencesStore.language === "zh" ? "插入" : "Insert"}
              data-keyboard={menuKeyboardOpen}
              use:editorPopover={{ close: closeDiagramMenu, trigger: diagramMenuTrigger }}
            >
              {#if diagramMenuView === "root"}
                <p class="editor-popover-title">
                  {$preferencesStore.language === "zh" ? "插入" : "Insert"}
                </p>
                <button
                  class="editor-popover-row editor-diagram-row"
                  type="button"
                  role="menuitem"
                  disabled={imageBusy}
                  on:click={runImageInsert}
                >
                  <ImageIcon aria-hidden="true" size={17} strokeWidth={1.8} />
                  <span>{imageBusy ? t("fontUploading", $preferencesStore.language) : t("insertImage", $preferencesStore.language)}</span>
                </button>
                <button
                  class="editor-popover-row editor-diagram-row"
                  type="button"
                  role="menuitem"
                  on:click={() => void showDiagramTemplates("mermaid")}
                >
                  <Network aria-hidden="true" size={17} strokeWidth={1.8} />
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
                  on:click={() => void showDiagramTemplates("root")}
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
          aria-keyshortcuts="Control+/ Meta+/"
          disabled={contentLocked}
          on:click={toggleSourceMode}
        >
          <FileCode aria-hidden="true" size={19} strokeWidth={2} />
        </button>
        </div>
        <button
          class="editor-done-button"
          type="button"
          aria-label={t("done", $preferencesStore.language)}
          title={t("done", $preferencesStore.language)}
          disabled={finishing}
          on:click={() => void finishEditing()}
        >
          <Check aria-hidden="true" size={20} strokeWidth={2} />
        </button>
        <div class="editor-action-menu">
          <button
            bind:this={actionMenuTrigger}
            class="editor-more-button"
            type="button"
            aria-label={t("moreActions", $preferencesStore.language)}
            aria-expanded={actionMenuOpen}
            aria-haspopup="menu"
            aria-controls={`editor-actions-${note.id}`}
            title={t("moreActions", $preferencesStore.language)}
            on:click={(event) => toggleMenu("actions", event)}
            on:keydown={(event) => openMenuWithKeyboard("actions", event)}
          >
            <Ellipsis aria-hidden="true" size={21} strokeWidth={1.8} />
          </button>
          {#if actionMenuOpen}
            <div
              id={`editor-actions-${note.id}`}
              class="editor-action-menu__content"
              role="menu"
              aria-label={t("moreActions", $preferencesStore.language)}
              data-keyboard={menuKeyboardOpen}
              use:editorPopover={{ close: closeActionMenu, trigger: actionMenuTrigger }}
            >
              <button
                class="editor-action-button"
                type="button"
                role="menuitemcheckbox"
                aria-checked={detailsOpen}
                on:click={() =>
                  void runMenuAction(() => {
                    detailsOpen = !detailsOpen;
                  })}
              >
                <Info aria-hidden="true" size={16} strokeWidth={1.8} />
                <span>{t("noteInfo", $preferencesStore.language)}</span>
                {#if detailsOpen}<Check class="editor-menu-check" aria-hidden="true" size={15} />{/if}
              </button>
              <button
                class="editor-action-button"
                type="button"
                role="menuitemcheckbox"
                aria-checked={focusMode}
                on:click={() =>
                  void runMenuAction(() => {
                    focusMode = !focusMode;
                  })}
              >
                <Focus aria-hidden="true" size={16} strokeWidth={1.8} />
                {focusMode
                  ? t("exitFocus", $preferencesStore.language)
                  : t("enterFocus", $preferencesStore.language)}
              </button>
              <hr class="editor-menu-divider" />
              <button
                class="editor-action-button"
                type="button"
                role="menuitem"
                on:click={() => void runMenuAction(exportCurrentNoteMarkdown)}
                disabled={contentLocked}
              >
                <Download aria-hidden="true" size={16} strokeWidth={1.8} />
                {t("exportMarkdown", $preferencesStore.language)}
              </button>
              <button
                class="editor-action-button"
                type="button"
                role="menuitem"
                disabled={contentLocked}
                on:click={() =>
                  void runMenuAction(async () => {
                    if (await flushDraft()) connectionPushOpen = true;
                  })}
              >
                <CloudUpload aria-hidden="true" size={15} strokeWidth={2} />
                {t("noteConnectionPush", $preferencesStore.language)}
              </button>
              <button
                class="editor-action-button"
                type="button"
                role="menuitem"
                on:click={() =>
                  void runMenuAction(() => {
                    protectionError = "";
                    protectionOpen = true;
                  })}
              >
                <Shield aria-hidden="true" size={16} strokeWidth={1.8} />
                {t("noteProtection", $preferencesStore.language)}
              </button>
                <button
                  class="editor-action-button"
                  type="button"
                  role="menuitem"
                  disabled={contentLocked}
                  on:click={() =>
                    void runMenuAction(() => {
                      shareOpen = true;
                    })}
                >
                  <ImageIcon aria-hidden="true" size={16} strokeWidth={1.8} />
                  {t("generateImage", $preferencesStore.language)}
                </button>
              <hr class="editor-menu-divider" />
              <button
                class="editor-action-button"
                type="button"
                role="menuitem"
                on:click={() => void runMenuAction(toggleTrash)}
              >
                {#if note.is_deleted}<RotateCcw aria-hidden="true" size={16} strokeWidth={1.8} />
                {:else}<Trash2 aria-hidden="true" size={16} strokeWidth={1.8} />{/if}
                {note.is_deleted ? t("restore", $preferencesStore.language) : t("trashNote", $preferencesStore.language)}
              </button>
              {#if note.is_deleted}
                <button
                  class="editor-action-button danger"
                  type="button"
                  role="menuitem"
                  on:click={() => void runMenuAction(deletePermanent)}
                >
                  <Trash2 aria-hidden="true" size={16} strokeWidth={1.8} />
                  {t("deleteForever", $preferencesStore.language)}
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
          bind:this={folderMenuTrigger}
          class="editor-meta-button editor-folder-button"
          type="button"
          aria-label={`${t("noteLocation", $preferencesStore.language)}: ${folderLabel}`}
          aria-expanded={folderMenuOpen}
          aria-haspopup="menu"
          aria-controls={`editor-folders-${note.id}`}
          on:click={(event) => toggleMenu("folder", event)}
          on:keydown={(event) => openMenuWithKeyboard("folder", event)}
        >
          <span>{folderLabel}</span>
          <span aria-hidden="true">⌄</span>
        </button>
        {#if folderMenuOpen}
          <div
            id={`editor-folders-${note.id}`}
            class="editor-popover editor-folder-popover"
            role="menu"
            aria-label={t("moveToNotebookGroup", $preferencesStore.language)}
            data-keyboard={menuKeyboardOpen}
            use:editorPopover={{ close: () => (folderMenuOpen = false), trigger: folderMenuTrigger }}
          >
            <p class="editor-popover-title">{t("moveToNotebookGroup", $preferencesStore.language)}</p>
            <button
              class:active={!note.folder_id}
              class="editor-popover-row"
              type="button"
              role="menuitemradio"
              aria-checked={!note.folder_id}
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
                  role="menuitemradio"
                  aria-checked={note.folder_id === option.id}
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
      <time class="editor-meta-text editor-note-date" datetime={note.updated_at}>{noteDate}</time>
      <span class="editor-meta-text">{wordCount} {t("wordUnit", $preferencesStore.language)}</span>
      <div class="editor-meta-popover-anchor editor-tag-anchor">
        <button
          bind:this={tagMenuTrigger}
          class="editor-meta-button editor-tags-button"
          type="button"
          aria-label={t("tags", $preferencesStore.language)}
          aria-expanded={tagMenuOpen}
          aria-controls={`editor-tags-${note.id}`}
          on:click={(event) => toggleMenu("tags", event)}
        >
          <TagIcon size={14} strokeWidth={1.8} aria-hidden="true" />
          <span>
            {visibleNoteTags.length
              ? `${visibleNoteTags[0].name}${hiddenNoteTagCount > 0 ? ` +${hiddenNoteTagCount}` : ""}`
              : t("tags", $preferencesStore.language)}
          </span>
        </button>
        {#if tagMenuOpen}
          <div
            id={`editor-tags-${note.id}`}
            class="editor-popover editor-tag-popover"
            role="group"
            aria-label={t("tags", $preferencesStore.language)}
            data-keyboard={menuKeyboardOpen}
            use:editorPopover={{ close: () => (tagMenuOpen = false), trigger: tagMenuTrigger }}
          >
            <p class="editor-popover-title">{t("tags", $preferencesStore.language)}</p>
            {#if note.tags?.length}
              <div class="editor-tag-chip-list" aria-label={t("currentTags", $preferencesStore.language)}>
                {#each note.tags as tag (tag.id)}
                  <button
                    type="button"
                    disabled={tagBusy}
                    title={tag.name}
                    aria-label={$preferencesStore.language === "zh" ? `移除标签 ${tag.name}` : `Remove tag ${tag.name}`}
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
        <Star aria-hidden="true" size={18} strokeWidth={1.8} fill={note.is_starred ? "currentColor" : "none"} />
      </button>
    </div>
    <div class:details-open={detailsOpen && !focusMode} class="editor-main">
      <div
        class="editor-surface"
        use:whiteboardRuntime={{ contentKey: draftContent, onOpen: openWhiteboard }}
      >
        {#if saveStatus === "error"}
          <div class="editor-save-error" role="alert">
            <p>{$preferencesStore.language === "zh" ? "暂未保存成功，你的修改仍保留在当前页面。请重试后再离开。" : "Your changes are still here, but could not be saved. Retry before leaving."}</p>
            <button type="button" on:click={() => void flushDraft()}>
              <RotateCw aria-hidden="true" size={15} strokeWidth={1.8} />
              {$preferencesStore.language === "zh" ? "重试保存" : "Retry save"}
            </button>
          </div>
        {/if}
        <textarea
          bind:this={titleInput}
          class="editor-title-input"
          value={draftTitle}
          placeholder={t("untitled", $preferencesStore.language)}
          aria-label={t("noteTitle", $preferencesStore.language)}
          rows="1"
          autocomplete="off"
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
            {#key note?.id}
            <svelte:component
              this={MarkdownEditor}
              value={draftContent}
              noteId={note?.id ?? ""}
              onChange={scheduleContentSave}
              bindEditor={bindMarkdownEditor}
            />
            {/key}
          {:else}
            <div class="markdown-editor-host markdown-editor-host--loading" role="status" aria-live="polite" aria-busy="true">
              <span class="editor-loading-message">{t("loading", $preferencesStore.language)}</span>
            </div>
          {/if}
        {/if}
      </div>
      {#if detailsOpen && !focusMode}
        <EditorInspector {note} onClose={() => {
          detailsOpen = false;
          actionMenuTrigger?.focus();
        }} />
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
        note={{ ...note, title: draftTitle, content: draftContent }}
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
      <div class="editor-empty__mark" aria-hidden="true">
        <FilePenLine size={28} strokeWidth={1.5} />
      </div>
      <p class="editor-empty-text">{t("selectOrCreate", $preferencesStore.language)}</p>
    </div>
  {/if}
</section>

<style>
  .editor-header {
    gap: 0.5rem;
  }

  .editor-header__context {
    display: flex;
    align-items: center;
    gap: 0.375rem;
    min-width: 0;
  }

  .editor-header__right {
    flex: 0 0 auto;
    max-width: none;
    gap: 0.25rem;
    overflow: visible;
  }

  .editor-writing-tools {
    display: flex;
    align-items: center;
    gap: 0.125rem;
    padding-right: 0.375rem;
    border-right: 1px solid var(--color-divider);
  }

  .editor-save-feedback {
    min-width: 0;
    font-family: var(--font-ui);
  }

  .editor-save-status {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    min-width: 0;
    font-size: 0.6875rem;
    white-space: nowrap;
    text-align: left;
  }

  .editor-save-status[data-state="pending"] {
    color: var(--color-text-sub);
  }

  .editor-save-retry {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    min-height: 36px;
    border: 0;
    border-radius: var(--radius-control);
    background: transparent;
    color: var(--color-danger);
    padding: 0 0.25rem;
    font-size: 0.6875rem;
    white-space: nowrap;
  }

  .editor-action-menu__content {
    width: min(17rem, calc(100vw - 1.5rem));
    max-height: min(70dvh, 36rem);
    overflow-y: auto;
    overscroll-behavior: contain;
  }

  .editor-action-menu__content .editor-action-button {
    gap: 0.625rem;
    min-height: 36px;
    height: auto;
    padding-block: 0.5rem;
    line-height: 1.4;
    white-space: normal;
  }

  .editor-action-menu__content :global(svg) {
    flex: 0 0 auto;
  }

  .editor-menu-divider {
    height: 1px;
    border: 0;
    background: var(--color-divider);
    margin: 0.25rem 0.5rem;
  }

  .editor-action-menu__content :global(.editor-menu-check) {
    margin-left: auto;
  }

  .editor-action-button[aria-checked="true"] {
    background: var(--color-surface-secondary);
    color: var(--color-ink-accent);
  }

  .editor-save-error {
    display: flex;
    align-items: flex-start;
    flex-wrap: wrap;
    gap: 0.5rem 1rem;
    margin-bottom: 1.5rem;
    border: 1px solid color-mix(in srgb, var(--color-danger) 25%, var(--color-divider));
    border-radius: var(--radius-control);
    background: color-mix(in srgb, var(--color-danger) 5%, var(--color-card));
    color: var(--color-danger);
    padding: 0.75rem;
    font: 0.8125rem/1.5 var(--font-ui);
  }

  .editor-save-error p {
    flex: 1 1 15rem;
    margin: 0;
  }

  .editor-save-error button {
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    min-height: 36px;
    border: 1px solid var(--color-divider);
    border-radius: var(--radius-control);
    background: var(--color-card);
    color: var(--color-danger);
    padding: 0.25rem 0.625rem;
    font-weight: 500;
  }

  .editor-loading-message {
    color: var(--color-text-sub);
    font: 0.8125rem/1.5 var(--font-ui);
  }

  .editor-popover[data-keyboard="true"],
  .editor-action-menu__content[data-keyboard="true"] {
    transition: none;
  }

  @starting-style {
    .editor-popover[data-keyboard="true"],
    .editor-action-menu__content[data-keyboard="true"] {
      opacity: 1;
      transform: none;
    }
  }

  @media (max-width: 760px) {
    .editor-image-button,
    .editor-note-date {
      display: none;
    }

    .editor-header {
      padding-inline: max(0.5rem, env(safe-area-inset-left)) max(0.5rem, env(safe-area-inset-right));
    }

    .editor-writing-tools {
      gap: 0;
      padding-right: 0.125rem;
    }

    .editor-header__right {
      gap: 0.125rem;
    }

    .editor-meta-bar {
      gap: 0.375rem;
      overflow: visible;
    }

    .editor-meta-popover-anchor {
      flex: 0 1 auto;
      max-width: min(7.25rem, 28vw);
    }

    .editor-folder-button,
    .editor-tags-button {
      max-width: 100%;
    }

    .editor-folder-popover,
    .editor-tag-popover,
    .editor-action-menu__content {
      left: 0.75rem;
      right: 0.75rem;
      width: auto;
    }

    .editor-save-status :global(svg) {
      display: none;
    }
  }

  @media (max-width: 960px) {
    .editor-main.details-open {
      grid-template-columns: minmax(0, 1fr);
      grid-template-rows: minmax(0, 1fr);
    }

    .editor-main.details-open .editor-surface,
    .showing-details .editor-writing-tools {
      display: none;
    }
  }

  @media (pointer: coarse) {
    .editor-action-menu__content .editor-action-button,
    .editor-save-retry,
    .editor-save-error button {
      min-height: 44px;
    }

    .editor-source-input,
    .editor-tag-add-row input {
      font-size: 1rem;
    }

    .editor-tag-add-row {
      grid-template-columns: minmax(0, 1fr) 44px;
    }

    .editor-tag-add-row button {
      width: 44px;
      height: 44px;
    }

    .editor-tag-add-row input,
    .editor-tag-chip-list button,
    .editor-tag-options button {
      min-height: 44px;
    }
  }
</style>
