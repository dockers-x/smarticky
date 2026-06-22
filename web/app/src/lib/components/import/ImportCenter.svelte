<script lang="ts">
  import type { ImportResult } from "../../api/imports";
  import { importsStore } from "../../stores/imports";
  import { preferencesStore, t } from "../../stores/preferences";
  import ImportSummary from "./ImportSummary.svelte";

  export let onBack: () => void = () => {};
  export let onImported: (result: ImportResult) => void | Promise<void> = () => {};
  export let showBack = true;

  let fileInput: HTMLInputElement;

  async function handleFileChange(event: Event): Promise<void> {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;

    await importsStore.preview(file);
    input.value = "";
  }

  async function confirmImport(): Promise<void> {
    const result = await importsStore.confirm();
    if (result && result.status !== "failed") {
      await onImported(result);
    }
  }
</script>

<div class="import-center">
  {#if showBack}
    <button class="import-center__back" type="button" on:click={onBack}>
      {t("back", $preferencesStore.language)}
    </button>
  {/if}

  <input
    bind:this={fileInput}
    class="visually-hidden"
    type="file"
    accept=".enex,.zip"
    aria-label={t("selectImportFile", $preferencesStore.language)}
    on:change={handleFileChange}
  />

  <div class="import-upload">
    <button
      type="button"
      disabled={$importsStore.loading}
      on:click={() => fileInput?.click()}
    >
      {t("selectFile", $preferencesStore.language)}
    </button>
    {#if $importsStore.fileName}
      <span title={$importsStore.fileName}>{$importsStore.fileName}</span>
    {/if}
  </div>

  {#if $importsStore.error}
    <p class="import-error" role="alert">{$importsStore.error}</p>
  {/if}

  {#if $importsStore.loading}
    <p class="import-muted" aria-live="polite">{t("importing", $preferencesStore.language)}</p>
  {/if}

  {#if $importsStore.preview && !$importsStore.result}
    <ImportSummary preview={$importsStore.preview} />
    <button
      class="import-primary-button"
      type="button"
      disabled={$importsStore.loading}
      on:click={confirmImport}
    >
      {t("importStart", $preferencesStore.language)}
    </button>
  {/if}

  {#if $importsStore.result}
    <ImportSummary result={$importsStore.result} />
    <div class="import-result-actions">
      <button type="button" on:click={() => importsStore.reset()}>
        {t("continueImport", $preferencesStore.language)}
      </button>
    </div>
  {/if}
</div>
