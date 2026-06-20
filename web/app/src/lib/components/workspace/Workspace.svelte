<script lang="ts">
  import { onMount } from "svelte";
  import EditorPane from "../editor/EditorPane.svelte";
  import { notesStore } from "../../stores/notes";
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
</div>
