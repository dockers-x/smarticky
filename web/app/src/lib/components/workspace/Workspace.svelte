<script lang="ts">
  import { onMount, tick } from "svelte";
  import { X } from "@lucide/svelte";
  import { dialog } from "../../actions/dialog";
  import EditorPane from "../editor/EditorPane.svelte";
  import ExcalidrawWhiteboardDialog from "../editor/ExcalidrawWhiteboardDialog.svelte";
  import ToolsPanel from "../settings/ToolsPanel.svelte";
  import { authStore } from "../../stores/auth";
  import { foldersStore } from "../../stores/folders";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";
  import { whiteboardStore } from "../../stores/whiteboard";
  import FolderBrowserPane from "./FolderBrowserPane.svelte";
  import IndexView from "./IndexView.svelte";
  import MobileNav from "./MobileNav.svelte";
  import NoteList from "./NoteList.svelte";
  import Sidebar from "./Sidebar.svelte";
  import TagBrowserPane from "./TagBrowserPane.svelte";

  let settingsOpen = false;
  let folderBrowserMode: "browse" | "move" | "create" = "browse";
  let selectedNoteIDs: string[] = [];
  let viewportWidth = 1440;
  let preferredNoteListWidth = 328;
  let noteListWidth = 328;
  let maxNoteListWidth = 460;
  let resizingList = false;
  let resizeStartX = 0;
  let resizeStartWidth = 328;
  let previousSelectedID: string | undefined;

  const noteListWidthStorageKey = "smarticky-note-list-width";
  const defaultNoteListWidth = 328;
  const minimumNoteListWidth = 280;
  const maximumNoteListWidth = 460;
  const minimumEditorWidth = 420;
  const resizeHandleWidth = 6;

  $: maxNoteListWidth = Math.max(
    minimumNoteListWidth,
    Math.min(
      maximumNoteListWidth,
      viewportWidth -
        ($preferencesStore.sidebarCompact ? 56 : 224) -
        minimumEditorWidth -
        resizeHandleWidth,
    ),
  );
  $: noteListWidth = Math.min(
    Math.max(preferredNoteListWidth, minimumNoteListWidth),
    maxNoteListWidth,
  );
  $: if ($notesStore.selected?.id !== previousSelectedID) {
    const closedNoteID = previousSelectedID;
    previousSelectedID = $notesStore.selected?.id;
    if (!previousSelectedID && closedNoteID && viewportWidth <= 960) {
      void restoreNoteFocus(closedNoteID);
    }
  }

  onMount(() => {
    void Promise.all([
      notesStore.load(),
      notesStore.loadCalendarNotes(),
      foldersStore.load(),
    ]);

    viewportWidth = window.innerWidth;
    const storedWidth = Number(localStorage.getItem(noteListWidthStorageKey));
    if (Number.isFinite(storedWidth) && storedWidth > 0) {
      preferredNoteListWidth = storedWidth;
    }

    const handleViewportResize = () => {
      viewportWidth = window.innerWidth;
    };
    window.addEventListener("resize", handleViewportResize);

    return () => {
      window.removeEventListener("resize", handleViewportResize);
      document.body.classList.remove("is-resizing-panels");
    };
  });

  function saveNoteListWidth(): void {
    localStorage.setItem(noteListWidthStorageKey, String(Math.round(preferredNoteListWidth)));
  }

  function setNoteListWidth(width: number, persist = false): void {
    const nextPreferredWidth = Math.min(
      Math.max(width, minimumNoteListWidth),
      maximumNoteListWidth,
    );
    const nextRenderedWidth = Math.min(nextPreferredWidth, maxNoteListWidth);
    if (
      nextRenderedWidth === noteListWidth &&
      preferredNoteListWidth !== noteListWidth
    ) {
      return;
    }
    preferredNoteListWidth = nextPreferredWidth;
    if (persist) saveNoteListWidth();
  }

  function startNoteListResize(event: PointerEvent): void {
    if (!event.isPrimary || event.button !== 0) return;
    event.preventDefault();
    (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
    resizingList = true;
    resizeStartX = event.clientX;
    resizeStartWidth = noteListWidth;
    document.body.classList.add("is-resizing-panels");
  }

  function updateNoteListResize(event: PointerEvent): void {
    if (!resizingList || !event.isPrimary) return;
    setNoteListWidth(resizeStartWidth + event.clientX - resizeStartX);
  }

  function finishNoteListResize(event?: PointerEvent): void {
    if (!resizingList) return;
    resizingList = false;
    document.body.classList.remove("is-resizing-panels");
    if (event) {
      const handle = event.currentTarget as HTMLElement;
      if (handle.hasPointerCapture(event.pointerId)) {
        handle.releasePointerCapture(event.pointerId);
      }
    }
    saveNoteListWidth();
  }

  function resetNoteListWidth(): void {
    preferredNoteListWidth = defaultNoteListWidth;
    saveNoteListWidth();
  }

  function handleResizeKeydown(event: KeyboardEvent): void {
    if (event.key === "ArrowLeft") {
      event.preventDefault();
      setNoteListWidth(noteListWidth - 16, true);
    } else if (event.key === "ArrowRight") {
      event.preventDefault();
      setNoteListWidth(noteListWidth + 16, true);
    } else if (event.key === "Home") {
      event.preventDefault();
      setNoteListWidth(minimumNoteListWidth, true);
    } else if (event.key === "End") {
      event.preventDefault();
      setNoteListWidth(maxNoteListWidth, true);
    }
  }

  function openFolderBrowser(mode: "browse" | "move" | "create" = "browse"): void {
    folderBrowserMode = mode;
    notesStore.showFolderBrowser({ preserveContext: mode !== "browse" });
  }

  function openTagBrowser(): void {
    folderBrowserMode = "browse";
    notesStore.showTagBrowser();
  }

  function closeBrowsers(): void {
    folderBrowserMode = "browse";
    notesStore.closeBrowsers();
  }

  async function restoreNoteFocus(noteID: string): Promise<void> {
    await tick();
    if (
      $notesStore.selected ||
      $notesStore.workspaceView !== "notes" ||
      document.querySelector('[role="dialog"][aria-modal="true"]')
    ) return;
    const card = Array.from(document.querySelectorAll<HTMLElement>("[data-note-id]"))
      .find((element) => element.dataset.noteId === noteID);
    card?.querySelector<HTMLButtonElement>(".note-card__body")?.focus({ preventScroll: true });
  }

  async function focusSearch(): Promise<void> {
    if ($notesStore.workspaceView !== "notes") await notesStore.setWorkspaceView("notes");
    if (viewportWidth <= 960) notesStore.clearSelection();
    await tick();
    const input = document.querySelector<HTMLInputElement>("#note-search");
    input?.focus();
    input?.select();
  }

  function handleSearchShortcut(event: KeyboardEvent): void {
    if (
      event.key !== "/" || event.defaultPrevented || event.isComposing ||
      event.ctrlKey || event.metaKey || event.altKey ||
      document.querySelector('[role="dialog"][aria-modal="true"]')
    ) return;
    const target = event.target;
    if (target instanceof HTMLElement && target.closest(
      'input, textarea, select, [contenteditable]:not([contenteditable="false"]), [role="textbox"]',
    )) return;
    event.preventDefault();
    void focusSearch();
  }
</script>

<svelte:window on:keydown={handleSearchShortcut} />

<main
  class:index-open={$notesStore.workspaceView === "index"}
  class:resizing-list={resizingList}
  class="workspace"
  aria-label="Smarticky workspace"
  style:--note-list-width={`${noteListWidth}px`}
  style:grid-template-columns={viewportWidth <= 960 ? "minmax(0, 1fr)" : undefined}
  style:grid-template-rows={viewportWidth <= 960
    ? $notesStore.selected ? "minmax(0, 1fr)" : "auto minmax(0, 1fr)"
    : undefined}
>
  <h1 class="visually-hidden">Smarticky</h1>
  <Sidebar
    {settingsOpen}
    onOpenSettings={() => (settingsOpen = true)}
    onOpenFolderBrowser={() => openFolderBrowser("browse")}
    onOpenTagBrowser={openTagBrowser}
    onSearch={() => void focusSearch()}
  />
  <MobileNav
    {settingsOpen}
    onOpenSettings={() => (settingsOpen = true)}
    onOpenFolderBrowser={() => openFolderBrowser("browse")}
    onOpenTagBrowser={openTagBrowser}
  />
  {#if $notesStore.workspaceView === "index"}
    <IndexView />
  {:else}
    <NoteList bind:selectedNoteIDs onOpenFolderBrowser={openFolderBrowser} />
    <!-- svelte-ignore a11y_no_noninteractive_tabindex a11y_no_noninteractive_element_interactions -->
    <div
      class="workspace-note-list-resizer"
      role="separator"
      aria-label={t("resizeNoteList", $preferencesStore.language)}
      aria-orientation="vertical"
      aria-valuemin={minimumNoteListWidth}
      aria-valuemax={Math.round(maxNoteListWidth)}
      aria-valuenow={Math.round(noteListWidth)}
      title={t("resizeNoteList", $preferencesStore.language)}
      tabindex="0"
      on:dblclick={resetNoteListWidth}
      on:keydown={handleResizeKeydown}
      on:pointerdown={startNoteListResize}
      on:pointermove={updateNoteListResize}
      on:pointerup={finishNoteListResize}
      on:pointercancel={finishNoteListResize}
      on:lostpointercapture={() => finishNoteListResize()}
    ></div>
  {/if}
  <EditorPane note={$notesStore.selected} />
  {#if $notesStore.folderBrowserOpen || $notesStore.tagBrowserOpen}
    <div
      class="workspace-browser-backdrop"
      role="presentation"
    >
    <div
      class="workspace-browser-overlay"
      role="dialog"
      aria-modal="true"
      tabindex="-1"
      aria-label={$notesStore.folderBrowserOpen
        ? t("notebookGroups", $preferencesStore.language)
        : t("tags", $preferencesStore.language)}
      use:dialog={{ onDismiss: closeBrowsers, initialFocus: 'input[type="search"]' }}
    >
      <div class="workspace-browser-actions">
        <button type="button" on:click={closeBrowsers}>
          <X size={17} strokeWidth={1.8} aria-hidden="true" />
          <span>{t("closeBrowserPane", $preferencesStore.language)}</span>
        </button>
      </div>
      {#if $notesStore.folderBrowserOpen}
        <FolderBrowserPane
          mode={folderBrowserMode}
          {selectedNoteIDs}
          onSelectionMoved={() => (selectedNoteIDs = [])}
        />
      {:else if $notesStore.tagBrowserOpen}
        <TagBrowserPane />
      {/if}
    </div>
    </div>
  {/if}
  {#if $whiteboardStore.openID}
    <ExcalidrawWhiteboardDialog
      whiteboardID={$whiteboardStore.openID}
      onClose={() => whiteboardStore.close()}
    />
  {/if}
  {#if settingsOpen}
    <ToolsPanel user={$authStore.user} onClose={() => (settingsOpen = false)} />
  {/if}
</main>

<style>
  .workspace-browser-backdrop {
    background: var(--color-backdrop);
  }

  .workspace-browser-overlay {
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-height: calc(100% - 76px);
    height: min(560px, calc(var(--app-height, 100dvh) - 76px));
  }

  .workspace-browser-overlay :global(.folder-browser-pane),
  .workspace-browser-overlay :global(.tag-browser-pane) {
    flex: 1 1 auto;
    min-height: 0;
    height: auto;
  }

  .workspace-browser-actions {
    display: flex;
    flex: 0 0 auto;
    justify-content: flex-end;
  }

  .workspace-browser-actions button {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    min-height: 36px;
    border: 0;
    border-radius: var(--radius-control);
    background: var(--color-surface-secondary);
    padding: 0 10px;
    color: var(--color-text-secondary);
    font-size: 12px;
  }

  @media (max-width: 960px) {
    .workspace :global(.note-list-pane),
    .workspace :global(.index-view) {
      grid-column: 1;
      grid-row: 2;
    }

    .workspace :global(.editor-pane.has-note) {
      grid-column: 1;
      grid-row: 1;
      border-bottom: 0;
    }

    .workspace :global(.editor-pane:not(.has-note)) {
      display: none;
    }

    .workspace-browser-overlay,
    .workspace:has(:global(.sidebar.compact)) .workspace-browser-overlay {
      top: max(12px, env(safe-area-inset-top));
      right: max(12px, env(safe-area-inset-right));
      bottom: max(12px, env(safe-area-inset-bottom));
      left: max(12px, env(safe-area-inset-left));
      height: auto;
      max-height: calc(var(--app-height, 100dvh) - 24px);
    }

    .workspace-browser-actions button {
      min-height: 44px;
    }
  }
</style>
