<script lang="ts">
  import { onMount } from "svelte";
  import type { Note } from "../../api/types";
  import { attachmentsStore } from "../../stores/attachments";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";
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
      await Promise.all([notesStore.load(), notesStore.loadCalendarNotes()]);
      tagName = "";
    } catch (err) {
      error = err instanceof Error ? err.message : t("addTagFailed", $preferencesStore.language);
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
      error =
        err instanceof Error
          ? err.message
          : t("uploadAttachmentFailed", $preferencesStore.language);
    } finally {
      busy = false;
    }
  }
</script>

<aside class="editor-inspector" aria-label={t("noteInfo", $preferencesStore.language)}>
  <section class="inspector-section">
    <h2>{t("tags", $preferencesStore.language)}</h2>
    {#if note.tags?.length}
      <div class="tag-list">
        {#each note.tags as tag (tag.id)}
          <span class="tag-chip">{tag.name}</span>
        {/each}
      </div>
    {:else}
      <p class="inspector-muted">{t("noTags", $preferencesStore.language)}</p>
    {/if}
    <form class="inspector-add-row" on:submit|preventDefault={handleAddTag}>
      <input
        bind:value={tagName}
        type="text"
        placeholder={t("addTag", $preferencesStore.language)}
        aria-label={t("addTag", $preferencesStore.language)}
        disabled={busy}
      />
      <button type="submit" disabled={busy || !tagName.trim()}>
        {t("add", $preferencesStore.language)}
      </button>
    </form>
  </section>

  <section class="inspector-section">
    <h2>{t("attachments", $preferencesStore.language)}</h2>
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
      <p class="inspector-muted">{t("noAttachments", $preferencesStore.language)}</p>
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
      {t("addAttachment", $preferencesStore.language)}
    </button>
  </section>

  {#if error}
    <p class="inspector-error" role="alert">{error}</p>
  {/if}
</aside>
