<script lang="ts">
  import { onMount } from "svelte";
  import type { Note } from "../../api/types";
  import { attachmentsStore } from "../../stores/attachments";
  import { notesStore } from "../../stores/notes";
  import { tagsStore } from "../../stores/tags";

  export let note: Note;

  let activeNoteID = "";
  let tagName = "";
  let fileInput: HTMLInputElement;
  let busy = false;
  let error = "";

  onMount(() => {
    void tagsStore.load();
  });

  $: if (note.id !== activeNoteID) {
    activeNoteID = note.id;
    tagName = "";
    error = "";
    void attachmentsStore.load(note.id);
  }

  async function handleAddTag(): Promise<void> {
    const trimmed = tagName.trim();
    if (!trimmed || busy) return;
    if (
      note.tags?.some((tag) => tag.name.toLowerCase() === trimmed.toLowerCase())
    ) {
      tagName = "";
      return;
    }

    busy = true;
    error = "";
    try {
      await tagsStore.addToNote(note.id, trimmed);
      await notesStore.load();
      tagName = "";
    } catch (err) {
      error = err instanceof Error ? err.message : "添加标签失败";
    } finally {
      busy = false;
    }
  }

  async function handleUpload(): Promise<void> {
    const file = fileInput.files?.[0];
    if (!file || busy) return;

    busy = true;
    error = "";
    try {
      await attachmentsStore.upload(note.id, file);
      fileInput.value = "";
    } catch (err) {
      error = err instanceof Error ? err.message : "上传附件失败";
    } finally {
      busy = false;
    }
  }
</script>

<aside class="editor-inspector" aria-label="笔记信息">
  <section class="inspector-section">
    <h2>标签</h2>
    {#if note.tags?.length}
      <div class="tag-list">
        {#each note.tags as tag (tag.id)}
          <span class="tag-chip">{tag.name}</span>
        {/each}
      </div>
    {:else}
      <p class="inspector-muted">暂无标签</p>
    {/if}
    <form class="inspector-add-row" on:submit|preventDefault={handleAddTag}>
      <input
        bind:value={tagName}
        type="text"
        placeholder="添加标签"
        aria-label="添加标签"
        disabled={busy}
      />
      <button type="submit" disabled={busy || !tagName.trim()}>添加</button>
    </form>
  </section>

  <section class="inspector-section">
    <h2>附件</h2>
    {#if $attachmentsStore.length}
      <div class="attachment-list">
        {#each $attachmentsStore as attachment (attachment.id)}
          <div class="attachment-item">
            <span>{attachment.filename}</span>
            <small>{Math.ceil(attachment.file_size / 1024)} KB</small>
          </div>
        {/each}
      </div>
    {:else}
      <p class="inspector-muted">暂无附件</p>
    {/if}
    <input
      bind:this={fileInput}
      type="file"
      class="visually-hidden"
      on:change={handleUpload}
    />
    <button
      class="inspector-secondary-button"
      type="button"
      disabled={busy}
      on:click={() => fileInput.click()}
    >
      添加附件
    </button>
  </section>

  {#if error}
    <p class="inspector-error" role="alert">{error}</p>
  {/if}
</aside>
