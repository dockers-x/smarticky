<script lang="ts">
  import { notify } from "../../stores/dialogs";
  import { foldersStore } from "../../stores/folders";
  import { preferencesStore, t } from "../../stores/preferences";

  let maxDepth = 3;
  let saving = false;
  let error = "";

  $: maxDepth = $foldersStore.settings.max_depth;

  async function save(): Promise<void> {
    if (saving) return;
    error = "";
    if (!Number.isInteger(maxDepth) || maxDepth < 1 || maxDepth > 50) {
      error = $preferencesStore.language === "zh" ? "请输入 1 到 50 之间的整数。" : "Enter a whole number from 1 to 50.";
      return;
    }
    saving = true;
    try {
      await foldersStore.saveSettings({ max_depth: maxDepth });
      notify(t("folderSettingsSaved", $preferencesStore.language), "success");
    } catch (saveError) {
      error = saveError instanceof Error ? saveError.message : t("folderSettingsFailed", $preferencesStore.language);
    } finally {
      saving = false;
    }
  }
</script>

<div class="settings-view">
  <section class="settings-section">
    <h3>{t("folderSettings", $preferencesStore.language)}</h3>
    <form class="settings-form" aria-busy={saving} on:submit|preventDefault={() => void save()}>
      <label>
        <span>{t("folderMaxDepth", $preferencesStore.language)}</span>
        <input
          bind:value={maxDepth}
          type="number"
          inputmode="numeric"
          required
          disabled={saving}
          min="1"
          max="50"
          step="1"
          aria-describedby="folder-max-depth-hint"
        />
        <small id="folder-max-depth-hint">{t("folderMaxDepthHint", $preferencesStore.language)}</small>
      </label>
      <div class="settings-actions">
        <button class="primary" type="submit" disabled={saving}>
          {t(saving ? "saving" : "saveSettings", $preferencesStore.language)}
        </button>
      </div>
      {#if error}<p class="settings-error" role="alert">{error}</p>{/if}
    </form>
  </section>
</div>
