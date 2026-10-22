<script lang="ts">
  import { onMount } from "svelte";
  import { FileUp, Type, Upload } from "@lucide/svelte";
  import { scrollRegion } from "../../actions/dialog";
  import type { User } from "../../api/types";
  import { confirmDialog, notify } from "../../stores/dialogs";
  import {
    DEFAULT_FONT,
    fontsStore,
    systemFontOptions,
  } from "../../stores/fonts";
  import { preferencesStore, t } from "../../stores/preferences";

  export let user: User | null = null;

  const maxFontSize = 30 * 1024 * 1024;
  const previewText = "The quick brown fox jumps over the lazy dog 我能吞下玻璃而不伤身体";

  let fileInput: HTMLInputElement | null = null;
  let selectedFile: File | null = null;
  let displayName = "";
  let isShared = true;
  let previewFamily = "";
  let uploadBusy = false;
  let uploadProgress = 0;
  let localPreviewURL = "";
  let previewError = "";
  let uploadError = "";
  let deletingFontID = "";
  let confirmingFontDeletion = false;

  function resetForm(): void {
    selectedFile = null;
    displayName = "";
    isShared = true;
    previewFamily = "";
    previewError = "";
    uploadError = "";
    uploadProgress = 0;
    if (localPreviewURL) {
      URL.revokeObjectURL(localPreviewURL);
      localPreviewURL = "";
    }
    if (fileInput) fileInput.value = "";
  }

  async function handleFileChange(event: Event): Promise<void> {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0] ?? null;
    uploadError = "";
    if (!file) {
      resetForm();
      return;
    }

    if (file.size > maxFontSize) {
      resetForm();
      uploadError = t("fontFileTooLarge", $preferencesStore.language);
      return;
    }

    if (!/\.(ttf|otf|woff|woff2)$/i.test(file.name)) {
      resetForm();
      uploadError = t("fontFormatInvalid", $preferencesStore.language);
      return;
    }

    selectedFile = file;
    previewFamily = "";
    previewError = "";
    uploadProgress = 0;
    if (!displayName) {
      displayName = file.name.replace(/\.(ttf|otf|woff|woff2)$/i, "");
    }

    if (typeof FontFace !== "undefined") {
      if (localPreviewURL) URL.revokeObjectURL(localPreviewURL);
      localPreviewURL = URL.createObjectURL(file);
      const family = `preview-${Date.now()}`;
      const fontFace = new FontFace(family, `url(${localPreviewURL})`);
      try {
        await fontFace.load();
        document.fonts.add(fontFace);
        previewFamily = family;
      } catch {
        previewError = t("fontPreviewUnavailable", $preferencesStore.language);
        URL.revokeObjectURL(localPreviewURL);
        localPreviewURL = "";
      }
    } else {
      previewError = t("fontPreviewUnavailable", $preferencesStore.language);
    }
  }

  async function uploadSelectedFont(): Promise<void> {
    if (uploadBusy) return;
    uploadError = "";
    if (!selectedFile) {
      uploadError = t("fontFileRequired", $preferencesStore.language);
      return;
    }
    if (!displayName.trim()) {
      uploadError = t("displayNameRequired", $preferencesStore.language);
      return;
    }

    uploadBusy = true;
    uploadProgress = 0;
    try {
      await fontsStore.upload({
        file: selectedFile,
        displayName: displayName.trim(),
        isShared,
        previewText,
        onProgress: (progress) => {
          uploadProgress = progress;
        },
      });
      notify(t("fontUploadSuccess", $preferencesStore.language), "success");
      resetForm();
    } catch (error) {
      uploadProgress = 0;
      uploadError = error instanceof Error
          ? error.message
          : t("fontUploadFailed", $preferencesStore.language);
    } finally {
      uploadBusy = false;
    }
  }

  async function deleteSelectedFont(fontID: string, name: string): Promise<void> {
    if (deletingFontID || confirmingFontDeletion) return;
    confirmingFontDeletion = true;
    const confirmed = await confirmDialog({
      title: t("delete", $preferencesStore.language),
      message: `${t("fontDeleteConfirm", $preferencesStore.language)} ${name}?`,
      confirmLabel: t("delete", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
    });
    confirmingFontDeletion = false;
    if (!confirmed) return;

    deletingFontID = fontID;
    try {
      await fontsStore.delete(fontID);
      notify(t("fontDeleteSuccess", $preferencesStore.language), "success");
    } catch (error) {
      notify(
        error instanceof Error
          ? error.message
          : t("fontDeleteFailed", $preferencesStore.language),
        "error",
      );
    } finally {
      deletingFontID = "";
    }
  }

  function canDelete(uploaderID: number): boolean {
    return Boolean(user && (user.role === "admin" || user.id === uploaderID));
  }

  function formatFileSize(bytes: number): string {
    if (!bytes) return "0 B";
    const units = ["B", "KB", "MB", "GB"];
    const index = Math.min(
      Math.floor(Math.log(bytes) / Math.log(1024)),
      units.length - 1,
    );
    return `${Math.round((bytes / 1024 ** index) * 100) / 100} ${units[index]}`;
  }

  onMount(() => {
    void fontsStore.load();
    return () => {
      if (localPreviewURL) URL.revokeObjectURL(localPreviewURL);
    };
  });
</script>

<div class="settings-view">
  <section class="settings-section">
    <h3>{t("selectedFont", $preferencesStore.language)}</h3>
    <div class="settings-form">
    <label>
      <span>{t("fontManagement", $preferencesStore.language)}</span>
      <select
        value={$fontsStore.selected}
        on:change={(event) => fontsStore.select(event.currentTarget.value)}
      >
        <optgroup label={t("systemFonts", $preferencesStore.language)}>
          {#each systemFontOptions as option}
            <option value={option.name}>
              {option.name === DEFAULT_FONT
                ? t("systemDefault", $preferencesStore.language)
                : option.displayName}
            </option>
          {/each}
        </optgroup>
        {#if $fontsStore.fonts.length}
          <optgroup label={t("uploadedFonts", $preferencesStore.language)}>
            {#each $fontsStore.fonts as font (font.id)}
              <option value={font.name}>{font.display_name}</option>
            {/each}
          </optgroup>
        {/if}
      </select>
    </label>
    </div>
  </section>

  <section class="settings-section">
    <h3>{t("uploadFont", $preferencesStore.language)}</h3>
    <form class="settings-form" aria-busy={uploadBusy} on:submit|preventDefault={() => void uploadSelectedFont()}>
      <div class="settings-field">
        <span>{t("fontFile", $preferencesStore.language)}</span>
        <div class="font-upload-control">
          <input
            bind:this={fileInput}
            id="settings-font-file"
            class="font-upload-input"
            accept=".ttf,.otf,.woff,.woff2"
            type="file"
            aria-label={t("fontFile", $preferencesStore.language)}
            aria-describedby="settings-font-file-hint"
            disabled={uploadBusy}
            tabindex="-1"
            on:change={handleFileChange}
          />
          <button
            class="font-upload-button"
            type="button"
            aria-controls="settings-font-file"
            disabled={uploadBusy}
            on:click={() => fileInput?.click()}
          >
            <FileUp size={16} strokeWidth={1.8} aria-hidden="true" />
            {selectedFile
              ? t("fontChangeFile", $preferencesStore.language)
              : t("fontChooseFile", $preferencesStore.language)}
          </button>
          {#if selectedFile}
            <div class="font-upload-meta" aria-live="polite">
              <FileUp size={16} strokeWidth={1.8} aria-hidden="true" />
              <div>
                <strong title={selectedFile.name}>{selectedFile.name}</strong>
                <span>{t("fontSelectedFile", $preferencesStore.language)} · {formatFileSize(selectedFile.size)}</span>
              </div>
            </div>
          {/if}
        </div>
        <small id="settings-font-file-hint">{t("fontFileHint", $preferencesStore.language)}</small>
      </div>
      <label>
        <span>{t("fontDisplayName", $preferencesStore.language)}</span>
        <input
          bind:value={displayName}
          disabled={uploadBusy}
          required
          type="text"
          placeholder={t("fontDisplayNamePlaceholder", $preferencesStore.language)}
        />
      </label>
      <label class="settings-switch-row">
        <span>
          <strong>{t("shareWithAllUsers", $preferencesStore.language)}</strong>
          <small>{t("fontPrivateHint", $preferencesStore.language)}</small>
        </span>
        <input bind:checked={isShared} type="checkbox" disabled={uploadBusy} />
      </label>
      {#if previewFamily || previewError}
        <div class="font-preview-card">
          <span>
            <Type size={15} strokeWidth={1.8} aria-hidden="true" />
            {t("fontLocalPreview", $preferencesStore.language)}
          </span>
          {#if previewFamily}
            <div class="font-preview" style={`font-family: "${previewFamily}";`}>
              {previewText}
            </div>
          {:else}
            <p>{previewError}</p>
          {/if}
        </div>
      {/if}
      {#if uploadBusy}
        <div
          class="font-upload-progress"
          role="progressbar"
          aria-label={t("fontUploadProgress", $preferencesStore.language)}
          aria-valuemin="0"
          aria-valuemax="100"
          aria-valuenow={uploadProgress}
        >
          <div>
            <span>{t("fontUploadProgress", $preferencesStore.language)}</span>
            <strong>{uploadProgress}%</strong>
          </div>
          <div class="font-upload-progress__bar" aria-hidden="true">
            <span style={`width: ${uploadProgress}%`}></span>
          </div>
        </div>
      {/if}
      <div class="settings-actions">
        <button
          class="primary"
          type="submit"
          disabled={uploadBusy}
        >
          <Upload size={15} strokeWidth={1.8} aria-hidden="true" />
          {uploadBusy
            ? t("fontUploading", $preferencesStore.language)
            : t("uploadFontButton", $preferencesStore.language)}
        </button>
      </div>
      {#if uploadError}<p class="settings-error" role="alert">{uploadError}</p>{/if}
    </form>
  </section>

  <section class="settings-section">
    <div class="settings-section__header">
      <h3>{t("uploadedFonts", $preferencesStore.language)}</h3>
      <button type="button" disabled={$fontsStore.loading} on:click={() => fontsStore.load()}>
        {t("refresh", $preferencesStore.language)}
      </button>
    </div>
    {#if $fontsStore.loading}
      <p class="settings-muted" role="status">{t("loading", $preferencesStore.language)}</p>
    {:else if $fontsStore.error}
      <p class="settings-error" role="alert">{$fontsStore.error}</p>
    {:else if $fontsStore.fonts.length === 0}
      <p class="settings-empty">{t("noFonts", $preferencesStore.language)}</p>
    {:else}
      <div class="settings-table-wrap" role="region" aria-label={t("uploadedFonts", $preferencesStore.language)} use:scrollRegion>
        <table class="settings-table">
          <thead>
            <tr>
              <th scope="col">{t("fontName", $preferencesStore.language)}</th>
              <th scope="col">{t("fontPreview", $preferencesStore.language)}</th>
              <th scope="col">{t("format", $preferencesStore.language)}</th>
              <th scope="col">{t("size", $preferencesStore.language)}</th>
              <th scope="col">{t("uploadedBy", $preferencesStore.language)}</th>
              <th scope="col">{t("shared", $preferencesStore.language)}</th>
              <th scope="col">{t("actions", $preferencesStore.language)}</th>
            </tr>
          </thead>
          <tbody>
            {#each $fontsStore.fonts as font (font.id)}
              <tr>
                <td title={font.display_name}>{font.display_name}</td>
                <td>
                  <span class="font-preview-inline" style={`font-family: "${font.name}";`}>
                    {font.preview_text}
                  </span>
                </td>
                <td>{font.format.toUpperCase()}</td>
                <td>{formatFileSize(font.file_size)}</td>
                <td>{font.uploaded_by}</td>
                <td>{font.is_shared ? t("yes", $preferencesStore.language) : t("no", $preferencesStore.language)}</td>
                <td>
                  <div class="settings-row-actions">
                    <button type="button" on:click={() => fontsStore.select(font.name)}>
                      {t($fontsStore.selected === font.name ? "selectedFont" : "apply", $preferencesStore.language)}
                    </button>
                    {#if canDelete(font.uploader_id)}
                      <button
                        class="danger"
                        type="button"
                        disabled={Boolean(deletingFontID)}
                        aria-label={`${t("delete", $preferencesStore.language)} ${font.display_name}`}
                        on:click={() => deleteSelectedFont(font.id, font.display_name)}
                      >
                        {t("delete", $preferencesStore.language)}
                      </button>
                    {/if}
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </section>
</div>
