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

  onMount(() => {
    void Promise.all([
      notesStore.load(),
      notesStore.loadCalendarNotes(),
      foldersStore.load(),
    ]);
  });

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

<div class:index-open={$notesStore.workspaceView === "index"} class="workspace">
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
</div>
