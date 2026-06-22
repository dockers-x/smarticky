<script lang="ts">
  import { onMount } from "svelte";
  import EditorPane from "../editor/EditorPane.svelte";
  import ExcalidrawWhiteboardDialog from "../editor/ExcalidrawWhiteboardDialog.svelte";
  import { notesStore } from "../../stores/notes";
  import { whiteboardStore } from "../../stores/whiteboard";
  import NoteList from "./NoteList.svelte";
  import Sidebar from "./Sidebar.svelte";

  onMount(() => {
    notesStore.load();
  });
</script>

<div class="workspace">
  <Sidebar />
  <NoteList />
  <EditorPane note={$notesStore.selected} />
  {#if $whiteboardStore.openID}
    <ExcalidrawWhiteboardDialog
      whiteboardID={$whiteboardStore.openID}
      onClose={() => whiteboardStore.close()}
    />
  {/if}
</div>
