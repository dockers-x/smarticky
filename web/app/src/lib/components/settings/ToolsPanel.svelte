<script lang="ts">
  import type { ImportResult } from "../../api/imports";
  import type { User } from "../../api/types";
  import ImportCenter from "../import/ImportCenter.svelte";
  import BackupPanel from "./BackupPanel.svelte";
  import FontPanel from "./FontPanel.svelte";
  import ProfilePanel from "./ProfilePanel.svelte";
  import UserManagementPanel from "./UserManagementPanel.svelte";
  import { authStore } from "../../stores/auth";
  import { confirmDialog, notify } from "../../stores/dialogs";
  import { importsStore } from "../../stores/imports";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t, type MessageKey } from "../../stores/preferences";

  export let user: User | null = null;
  export let onClose: () => void = () => {};

  type ToolsView = "menu" | "import" | "backup" | "fonts" | "profile" | "users";

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

  function openView(nextView: ToolsView): void {
    view = nextView;
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
      action: () => openView("backup"),
      keepOpen: true,
    },
    {
      labelKey: "fontManagement",
      action: () => openView("fonts"),
      keepOpen: true,
    },
    {
      labelKey: "personalProfile",
      action: () => openView("profile"),
      keepOpen: true,
    },
    {
      labelKey: "userManagement",
      adminOnly: true,
      action: () => openView("users"),
      keepOpen: true,
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
  $: panelTitle =
    view === "menu"
      ? t("settings", $preferencesStore.language)
      : view === "import"
        ? t("import", $preferencesStore.language)
        : view === "backup"
          ? t("backupTitle", $preferencesStore.language)
          : view === "fonts"
            ? t("fontManagement", $preferencesStore.language)
            : view === "profile"
              ? t("personalProfile", $preferencesStore.language)
              : t("userManagement", $preferencesStore.language);
</script>

<section class="tools-panel" class:expanded-view={view !== "menu"} aria-label={t("settings", $preferencesStore.language)}>
  <div class="tools-panel__header">
    {#if view !== "menu"}
      <button class="tools-panel__back" type="button" aria-label={t("back", $preferencesStore.language)} on:click={() => (view = "menu")}>‹</button>
    {/if}
    <h2>{panelTitle}</h2>
    <button type="button" aria-label={t("closeSettings", $preferencesStore.language)} on:click={onClose}>×</button>
  </div>
  {#if view === "import"}
    <ImportCenter onBack={() => (view = "menu")} onImported={handleImported} />
  {:else if view === "backup"}
    <BackupPanel />
  {:else if view === "fonts"}
    <FontPanel {user} />
  {:else if view === "profile"}
    <ProfilePanel {user} />
  {:else if view === "users"}
    <UserManagementPanel {user} />
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
