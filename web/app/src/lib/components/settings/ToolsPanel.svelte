<script lang="ts">
  import type { ImportResult } from "../../api/imports";
  import type { User } from "../../api/types";
  import ImportCenter from "../import/ImportCenter.svelte";
  import { authStore } from "../../stores/auth";
  import { confirmDialog, notify } from "../../stores/dialogs";
  import { importsStore } from "../../stores/imports";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t, type MessageKey } from "../../stores/preferences";

  export let user: User | null = null;
  export let onClose: () => void = () => {};

  type ToolsView = "menu" | "import";

  interface ToolRow {
    labelKey: MessageKey;
    action: () => void | Promise<void>;
    danger?: boolean;
    adminOnly?: boolean;
    keepOpen?: boolean;
  }

  let view: ToolsView = "menu";

  function openImport(): void {
    importsStore.reset();
    view = "import";
  }

  async function handleImported(result: ImportResult): Promise<void> {
    await notesStore.setFilter("all");
    await notesStore.setSearch("");
    notify(
      result.failed_count > 0
        ? t("importCompletedPartial", $preferencesStore.language)
        : t("importCompleted", $preferencesStore.language),
      result.failed_count > 0 ? "info" : "success",
    );
  }

  const rows: ToolRow[] = [
    {
      labelKey: "import",
      action: openImport,
      keepOpen: true,
    },
    {
      labelKey: "backup",
      action: () => notify(t("backupComing", $preferencesStore.language), "info"),
    },
    {
      labelKey: "fontManagement",
      action: () =>
        notify(t("fontManagementComing", $preferencesStore.language), "info"),
    },
    {
      labelKey: "personalProfile",
      action: () =>
        notify(t("personalProfileComing", $preferencesStore.language), "info"),
    },
    {
      labelKey: "userManagement",
      adminOnly: true,
      action: () =>
        notify(t("userManagementComing", $preferencesStore.language), "info"),
    },
    {
      labelKey: "logout",
      danger: true,
      action: async () => {
        const confirmed = await confirmDialog({
          title: t("logout", $preferencesStore.language),
          message: t("logoutConfirm", $preferencesStore.language),
          confirmLabel: t("logout", $preferencesStore.language),
          cancelLabel: t("cancel", $preferencesStore.language),
        });
        if (confirmed) authStore.logout();
      },
    },
  ];

  $: visibleRows = rows.filter((row) => !row.adminOnly || user?.role === "admin");
</script>

<section class="tools-panel" class:import-view={view === "import"} aria-label={t("settings", $preferencesStore.language)}>
  <div class="tools-panel__header">
    <h2>{view === "import" ? t("import", $preferencesStore.language) : t("settings", $preferencesStore.language)}</h2>
    <button type="button" aria-label={t("closeSettings", $preferencesStore.language)} on:click={onClose}>×</button>
  </div>
  {#if view === "import"}
    <ImportCenter onBack={() => (view = "menu")} onImported={handleImported} />
  {:else}
    <div class="tools-list">
      {#each visibleRows as row}
        <button
          class:danger={row.danger}
          type="button"
          on:click={() => {
            void row.action();
            if (!row.danger && !row.keepOpen) onClose();
          }}
        >
          <span>{t(row.labelKey, $preferencesStore.language)}</span>
          <span aria-hidden="true">›</span>
        </button>
      {/each}
    </div>
  {/if}
</section>
