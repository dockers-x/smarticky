<script lang="ts">
  import { onMount } from "svelte";
  import EditorPane from "../editor/EditorPane.svelte";
  import ExcalidrawWhiteboardDialog from "../editor/ExcalidrawWhiteboardDialog.svelte";
  import { foldersStore } from "../../stores/folders";
  import { notesStore } from "../../stores/notes";
  import { whiteboardStore } from "../../stores/whiteboard";
  import IndexView from "./IndexView.svelte";
  import MobileNav from "./MobileNav.svelte";
  import NoteList from "./NoteList.svelte";
  import Sidebar from "./Sidebar.svelte";

  onMount(() => {
    notesStore.load();
    foldersStore.load();
  });
</script>

<div class:index-open={$notesStore.workspaceView === "index"} class="workspace">
  <Sidebar />
  <MobileNav />
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
</div>
