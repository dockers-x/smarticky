<script lang="ts">
  import { onMount } from "svelte";
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
  let localPreviewURL = "";

  function resetForm(): void {
    selectedFile = null;
    displayName = "";
    isShared = true;
    previewFamily = "";
    if (localPreviewURL) {
      URL.revokeObjectURL(localPreviewURL);
      localPreviewURL = "";
    }
    if (fileInput) fileInput.value = "";
  }

  async function handleFileChange(event: Event): Promise<void> {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0] ?? null;
    if (!file) {
      resetForm();
      return;
    }

    if (file.size > maxFontSize) {
      notify(t("fontFileTooLarge", $preferencesStore.language), "error");
      input.value = "";
      return;
    }

    if (!/\.(ttf|otf|woff|woff2)$/i.test(file.name)) {
      notify(t("fontFormatInvalid", $preferencesStore.language), "error");
      input.value = "";
      return;
    }

    selectedFile = file;
    if (!displayName) {
      displayName = file.name.replace(/\.(ttf|otf|woff|woff2)$/i, "");
    }

    if (typeof FontFace !== "undefined") {
      if (localPreviewURL) URL.revokeObjectURL(localPreviewURL);
      localPreviewURL = URL.createObjectURL(file);
      const family = `preview-${Date.now()}`;
      const fontFace = new FontFace(family, `url(${localPreviewURL})`);
      await fontFace.load();
      document.fonts.add(fontFace);
      previewFamily = family;
    }
  }

  async function uploadSelectedFont(): Promise<void> {
    if (!selectedFile) {
      notify(t("fontFileRequired", $preferencesStore.language), "error");
      return;
    }
    if (!displayName.trim()) {
      notify(t("displayNameRequired", $preferencesStore.language), "error");
      return;
    }

    uploadBusy = true;
    try {
      await fontsStore.upload({
        file: selectedFile,
        displayName: displayName.trim(),
        isShared,
        previewText,
      });
      notify(t("fontUploadSuccess", $preferencesStore.language), "success");
      resetForm();
    } catch (error) {
      notify(
        error instanceof Error
          ? error.message
          : t("fontUploadFailed", $preferencesStore.language),
        "error",
      );
    } finally {
      uploadBusy = false;
    }
  }

  async function deleteSelectedFont(fontID: string, name: string): Promise<void> {
    const confirmed = await confirmDialog({
      title: t("delete", $preferencesStore.language),
      message: `${t("fontDeleteConfirm", $preferencesStore.language)} ${name}?`,
      confirmLabel: t("delete", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
    });
    if (!confirmed) return;

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
  </section>

  <section class="settings-section">
    <h3>{t("uploadFont", $preferencesStore.language)}</h3>
    <div class="settings-form">
      <label>
        <span>{t("fontFile", $preferencesStore.language)}</span>
        <input
          bind:this={fileInput}
          accept=".ttf,.otf,.woff,.woff2"
          type="file"
          on:change={handleFileChange}
        />
        <small>{t("fontFileHint", $preferencesStore.language)}</small>
      </label>
      <label>
        <span>{t("fontDisplayName", $preferencesStore.language)}</span>
        <input
          bind:value={displayName}
          type="text"
          placeholder={t("fontDisplayNamePlaceholder", $preferencesStore.language)}
        />
      </label>
      <label class="settings-switch-row">
        <span>
          <strong>{t("shareWithAllUsers", $preferencesStore.language)}</strong>
          <small>{t("fontPrivateHint", $preferencesStore.language)}</small>
        </span>
        <input bind:checked={isShared} type="checkbox" />
      </label>
      {#if previewFamily}
        <div class="font-preview" style={`font-family: "${previewFamily}";`}>
          {previewText}
        </div>
      {/if}
      <div class="settings-actions">
        <button
          class="primary"
          type="button"
          disabled={uploadBusy}
          on:click={uploadSelectedFont}
        >
          {t("uploadFontButton", $preferencesStore.language)}
        </button>
      </div>
    </div>
  </section>

  <section class="settings-section">
    <div class="settings-section__header">
      <h3>{t("uploadedFonts", $preferencesStore.language)}</h3>
      <button type="button" disabled={$fontsStore.loading} on:click={() => fontsStore.load()}>
        {t("refresh", $preferencesStore.language)}
      </button>
    </div>
    {#if $fontsStore.loading}
      <p class="settings-muted">{t("loading", $preferencesStore.language)}</p>
    {:else if $fontsStore.error}
      <p class="settings-error">{$fontsStore.error}</p>
    {:else if $fontsStore.fonts.length === 0}
      <p class="settings-empty">{t("noFonts", $preferencesStore.language)}</p>
    {:else}
      <div class="settings-table-wrap">
        <table class="settings-table">
          <thead>
            <tr>
              <th>{t("fontName", $preferencesStore.language)}</th>
              <th>{t("fontPreview", $preferencesStore.language)}</th>
              <th>{t("format", $preferencesStore.language)}</th>
              <th>{t("size", $preferencesStore.language)}</th>
              <th>{t("uploadedBy", $preferencesStore.language)}</th>
              <th>{t("shared", $preferencesStore.language)}</th>
              <th>{t("actions", $preferencesStore.language)}</th>
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
                      {t("apply", $preferencesStore.language)}
                    </button>
                    {#if canDelete(font.uploader_id)}
                      <button
                        class="danger"
                        type="button"
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
