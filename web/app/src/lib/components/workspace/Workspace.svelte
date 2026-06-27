<script lang="ts">
  import { onMount } from "svelte";
  import EditorPane from "../editor/EditorPane.svelte";
  import ExcalidrawWhiteboardDialog from "../editor/ExcalidrawWhiteboardDialog.svelte";
  import ToolsPanel from "../settings/ToolsPanel.svelte";
  import { authStore } from "../../stores/auth";
  import { foldersStore } from "../../stores/folders";
  import { notesStore } from "../../stores/notes";
  import { whiteboardStore } from "../../stores/whiteboard";
  import IndexView from "./IndexView.svelte";
  import MobileNav from "./MobileNav.svelte";
  import NoteList from "./NoteList.svelte";
  import Sidebar from "./Sidebar.svelte";

  let settingsOpen = false;

  onMount(() => {
    void Promise.all([
      notesStore.load(),
      notesStore.loadCalendarNotes(),
      foldersStore.load(),
    ]);
  });
</script>

<div class:index-open={$notesStore.workspaceView === "index"} class="workspace">
  <Sidebar {settingsOpen} onOpenSettings={() => (settingsOpen = true)} />
  <MobileNav {settingsOpen} onOpenSettings={() => (settingsOpen = true)} />
  {#if $notesStore.workspaceView === "index"}
    <IndexView />
  {:else}
    <NoteList />
  {/if}
  <EditorPane note={$notesStore.selected} />
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
