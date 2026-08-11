<script lang="ts">
  import { onMount } from "svelte";
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

  function handleWorkspaceKeydown(event: KeyboardEvent): void {
    if (event.key !== "Escape") return;
    if (!$notesStore.folderBrowserOpen && !$notesStore.tagBrowserOpen) return;
    closeBrowsers();
  }
</script>

<svelte:window on:keydown={handleWorkspaceKeydown} />

<main
  class:index-open={$notesStore.workspaceView === "index"}
  class:resizing-list={resizingList}
  class="workspace"
  aria-label="Smarticky workspace"
  style={`--note-list-width: ${noteListWidth}px`}
>
  <h1 class="visually-hidden">Smarticky</h1>
  <Sidebar
    {settingsOpen}
    onOpenSettings={() => (settingsOpen = true)}
    onOpenFolderBrowser={() => openFolderBrowser("browse")}
    onOpenTagBrowser={openTagBrowser}
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
    <button
      class="workspace-browser-backdrop"
      type="button"
      aria-label={t("closeBrowserPane", $preferencesStore.language)}
      on:click={closeBrowsers}
    ></button>
    <div
      class="workspace-browser-overlay"
      role="dialog"
      aria-label={$notesStore.folderBrowserOpen
        ? t("notebookGroups", $preferencesStore.language)
        : t("tags", $preferencesStore.language)}
    >
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
