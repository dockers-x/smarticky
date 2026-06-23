<script lang="ts">
  import { notify } from "../../stores/dialogs";
  import { foldersStore } from "../../stores/folders";
  import { preferencesStore, t } from "../../stores/preferences";

  let maxDepth = 3;
  let saving = false;

  $: maxDepth = $foldersStore.settings.max_depth;

  async function save(): Promise<void> {
    saving = true;
    try {
      await foldersStore.saveSettings({ max_depth: maxDepth });
      notify(t("folderSettingsSaved", $preferencesStore.language), "success");
    } catch {
      notify(t("folderSettingsFailed", $preferencesStore.language), "error");
    } finally {
      saving = false;
    }
  }
</script>

<div class="settings-view">
  <section class="settings-section">
    <h3>{t("folderSettings", $preferencesStore.language)}</h3>
    <form class="settings-form" on:submit|preventDefault={() => void save()}>
      <label>
        <span>{t("folderMaxDepth", $preferencesStore.language)}</span>
        <input
          bind:value={maxDepth}
          type="number"
          min="1"
          max="50"
          step="1"
        />
        <small>{t("folderMaxDepthHint", $preferencesStore.language)}</small>
      </label>
      <div class="settings-actions">
        <button class="primary" type="submit" disabled={saving}>
          {t("saveSettings", $preferencesStore.language)}
        </button>
      </div>
    </form>
  </section>
</div>
