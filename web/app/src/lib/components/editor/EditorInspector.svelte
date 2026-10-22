<script lang="ts">
  import { Download, Plus, X } from "@lucide/svelte";
  import { onMount } from "svelte";
  import type { Attachment, Note } from "../../api/types";
  import { fetchProtectedImageObjectURL } from "../../markdown/protectedImages";
  import { attachmentsStore } from "../../stores/attachments";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";
  import { tagsStore } from "../../stores/tags";

  export let note: Note;
  export let onClose: () => void = () => {};

  let activeNoteID = "";
  let tagName = "";
  let fileInput: HTMLInputElement;
  let busy = false;
  let error = "";
  let loading = false;
  let loadError = "";
  let downloadingID: number | null = null;

  onMount(() => {
    void tagsStore.load();
  });

  $: if (note.id !== activeNoteID) {
    activeNoteID = note.id;
    tagName = "";
    error = "";
    void loadAttachments(note.id);
  }

  async function loadAttachments(noteID: string): Promise<void> {
    loading = true;
    loadError = "";
    try {
      await attachmentsStore.load(noteID);
    } catch {
      if (activeNoteID === noteID) {
        loadError = $preferencesStore.language === "zh" ? "附件暂时无法加载" : "Attachments could not be loaded";
      }
    } finally {
      if (activeNoteID === noteID) loading = false;
    }
  }

  function formatDate(value: string): string {
    return new Date(value).toLocaleString($preferencesStore.language === "zh" ? "zh-CN" : "en-US", {
      dateStyle: "medium",
      timeStyle: "short",
      timeZone: $preferencesStore.timeZone,
    });
  }

  async function downloadAttachment(attachment: Attachment): Promise<void> {
    if (downloadingID !== null) return;
    downloadingID = attachment.id;
    error = "";
    try {
      const objectURL = await fetchProtectedImageObjectURL(`/api/attachments/${attachment.id}/download`);
      const link = document.createElement("a");
      link.href = objectURL;
      link.download = attachment.filename;
      link.click();
      URL.revokeObjectURL(objectURL);
    } catch {
      error = $preferencesStore.language === "zh" ? "附件下载失败，请重试。" : "The attachment could not be downloaded. Please retry.";
    } finally {
      downloadingID = null;
    }
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
    const noteID = note.id;
    try {
      await tagsStore.addToNote(noteID, trimmed);
      await Promise.all([notesStore.load(), notesStore.loadCalendarNotes()]);
      if (activeNoteID === noteID) tagName = "";
    } catch (err) {
      if (activeNoteID === noteID) error = err instanceof Error ? err.message : t("addTagFailed", $preferencesStore.language);
    } finally {
      busy = false;
    }
  }

  async function handleUpload(): Promise<void> {
    const file = fileInput.files?.[0];
    if (!file || busy) return;

    busy = true;
    error = "";
    const noteID = note.id;
    try {
      await attachmentsStore.upload(noteID, file);
      fileInput.value = "";
    } catch (err) {
      if (activeNoteID === noteID) error =
        err instanceof Error
          ? err.message
          : t("uploadAttachmentFailed", $preferencesStore.language);
    } finally {
      busy = false;
    }
  }
</script>

<aside class="editor-inspector" aria-label={t("noteInfo", $preferencesStore.language)}>
  <header class="inspector-header">
    <h2>{t("noteInfo", $preferencesStore.language)}</h2>
    <button type="button" class="inspector-close" aria-label={$preferencesStore.language === "zh" ? "关闭笔记信息" : "Close note info"} on:click={onClose}>
      <X size={17} strokeWidth={1.8} aria-hidden="true" />
    </button>
  </header>
  <dl class="inspector-dates">
    <div><dt>{$preferencesStore.language === "zh" ? "创建" : "Created"}</dt><dd><time datetime={note.created_at}>{formatDate(note.created_at)}</time></dd></div>
    <div><dt>{$preferencesStore.language === "zh" ? "修改" : "Updated"}</dt><dd><time datetime={note.updated_at}>{formatDate(note.updated_at)}</time></dd></div>
  </dl>
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
        autocomplete="off"
      />
      <button type="submit" disabled={busy || !tagName.trim()}>
        <Plus size={14} strokeWidth={1.8} aria-hidden="true" />
        {t("add", $preferencesStore.language)}
      </button>
    </form>
  </section>

  <section class="inspector-section">
    <h2>{t("attachments", $preferencesStore.language)}</h2>
    {#if loading}
      <p class="inspector-muted" role="status">{t("loading", $preferencesStore.language)}</p>
    {:else if loadError}
      <p class="inspector-error" role="alert">{loadError}</p>
      <button class="inspector-secondary-button" type="button" on:click={() => void loadAttachments(note.id)}>
        {$preferencesStore.language === "zh" ? "重试" : "Retry"}
      </button>
    {:else if $attachmentsStore.length}
      <div class="attachment-list">
        {#each $attachmentsStore as attachment (attachment.id)}
          <button
            class="attachment-item"
            type="button"
            disabled={downloadingID !== null}
            aria-busy={downloadingID === attachment.id}
            aria-label={$preferencesStore.language === "zh" ? `下载 ${attachment.filename}` : `Download ${attachment.filename}`}
            title={attachment.filename}
            on:click={() => void downloadAttachment(attachment)}
          >
            <span><Download size={14} strokeWidth={1.8} aria-hidden="true" />{attachment.filename}</span>
            <small>{Math.ceil(attachment.file_size / 1024)} KB</small>
          </button>
        {/each}
      </div>
    {:else}
      <p class="inspector-muted">{t("noAttachments", $preferencesStore.language)}</p>
    {/if}
    <input
      bind:this={fileInput}
      type="file"
      class="visually-hidden"
      aria-label={t("addAttachment", $preferencesStore.language)}
      tabindex="-1"
      on:change={handleUpload}
    />
    <button
      class="inspector-secondary-button"
      type="button"
      disabled={busy}
      on:click={() => fileInput.click()}
    >
      <Plus size={15} strokeWidth={1.8} aria-hidden="true" />
      {t("addAttachment", $preferencesStore.language)}
    </button>
  </section>

  {#if error}
    <p class="inspector-error" role="alert">{error}</p>
  {/if}
</aside>

<style>
  .editor-inspector {
    overscroll-behavior: contain;
    padding-bottom: max(1.25rem, env(safe-area-inset-bottom));
    font-family: var(--font-ui);
  }

  .inspector-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    margin: -0.375rem -0.375rem 1rem 0;
  }

  .inspector-header h2 {
    margin: 0;
    font-size: 0.875rem;
    font-weight: 600;
    letter-spacing: -0.01em;
  }

  .inspector-close {
    display: grid;
    place-items: center;
    min-width: 36px;
    min-height: 36px;
    border: 0;
    border-radius: var(--radius-control);
    background: transparent;
    color: var(--color-text-sub);
    padding: 0;
  }

  .inspector-dates {
    display: grid;
    gap: 0.625rem;
    margin: 0 0 1.5rem;
    padding-bottom: 1.25rem;
    border-bottom: 1px solid var(--color-divider);
    font-size: 0.75rem;
    line-height: 1.5;
  }

  .inspector-dates div {
    display: grid;
    gap: 0.125rem;
  }

  .inspector-dates dt {
    color: var(--color-text-sub);
  }

  .inspector-dates dd {
    margin: 0;
    color: var(--color-text-secondary);
    overflow-wrap: anywhere;
  }

  .inspector-section h2 {
    font-size: 0.75rem;
    font-weight: 600;
  }

  .inspector-add-row button,
  .inspector-secondary-button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 0.375rem;
    min-height: 36px;
    height: auto;
  }

  .inspector-add-row input {
    min-height: 36px;
  }

  .inspector-secondary-button {
    margin-top: 0.75rem;
  }

  .attachment-item {
    width: 100%;
    border: 0;
    border-bottom: 1px solid var(--color-divider);
    background: transparent;
    text-align: left;
    text-decoration: none;
  }

  .attachment-item span {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    overflow-wrap: anywhere;
    white-space: normal;
  }

  .attachment-item :global(svg) {
    flex: 0 0 auto;
  }

  .attachment-item small {
    padding-left: 1.375rem;
  }

  .inspector-close:active,
  .inspector-secondary-button:active:not(:disabled),
  .inspector-add-row button:active:not(:disabled) {
    background: var(--color-surface-muted);
  }

  @media (hover: hover) and (pointer: fine) {
    .inspector-close:hover {
      background: var(--color-surface-secondary);
      color: var(--color-text);
    }

    .attachment-item:hover span {
      color: var(--color-brand);
    }
  }

  @media (pointer: coarse) {
    .inspector-close,
    .inspector-add-row button,
    .inspector-secondary-button,
    .inspector-add-row input {
      min-height: 44px;
    }

    .inspector-close {
      min-width: 44px;
    }

    .inspector-add-row input {
      font-size: 1rem;
    }
  }
</style>
