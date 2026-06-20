<script lang="ts">
  import {
    DatabaseBackup,
    FileUp,
    LogOut,
    Type,
    User as UserIcon,
    Users,
    X,
  } from "@lucide/svelte";
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

  type ToolsView = "import" | "backup" | "fonts" | "profile" | "users";

  interface ToolNavItem {
    labelKey: MessageKey;
    view: ToolsView;
    adminOnly?: boolean;
  }

  let view: ToolsView = "profile";

  function selectView(nextView: ToolsView): void {
    if (nextView === "import") importsStore.reset();
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

  function handleKeydown(event: KeyboardEvent): void {
    if (event.key === "Escape") onClose();
  }

  async function logout(): Promise<void> {
    const confirmed = await confirmDialog({
      title: t("logout", $preferencesStore.language),
      message: t("logoutConfirm", $preferencesStore.language),
      confirmLabel: t("logout", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
    });
    if (confirmed) authStore.logout();
  }

  const navItems: ToolNavItem[] = [
    {
      labelKey: "import",
      view: "import",
    },
    {
      labelKey: "backup",
      view: "backup",
    },
    {
      labelKey: "fontManagement",
      view: "fonts",
    },
    {
      labelKey: "personalProfile",
      view: "profile",
    },
    {
      labelKey: "userManagement",
      adminOnly: true,
      view: "users",
    },
  ];

  $: visibleNavItems = navItems.filter((item) => !item.adminOnly || user?.role === "admin");
  $: panelTitle =
    view === "import"
      ? t("import", $preferencesStore.language)
      : view === "backup"
        ? t("backupTitle", $preferencesStore.language)
        : view === "fonts"
          ? t("fontManagement", $preferencesStore.language)
          : view === "profile"
            ? t("personalProfile", $preferencesStore.language)
            : t("userManagement", $preferencesStore.language);
</script>

<svelte:window on:keydown={handleKeydown} />

<div
  class="tools-panel-backdrop"
  role="presentation"
  on:click={(event) => {
    if (event.currentTarget === event.target) onClose();
  }}
>
  <div
    class="tools-panel"
    role="dialog"
    aria-modal="true"
    aria-labelledby="settings-dialog-title"
  >
    <div class="tools-panel__header">
      <div>
        <h2 id="settings-dialog-title">{t("settings", $preferencesStore.language)}</h2>
        <p>{panelTitle}</p>
      </div>
      <button class="tools-panel__close" type="button" aria-label={t("closeSettings", $preferencesStore.language)} on:click={onClose}>
        <X size={20} strokeWidth={1.8} aria-hidden="true" />
      </button>
    </div>

    <div class="tools-panel__body">
      <nav class="tools-panel__nav" aria-label={t("settings", $preferencesStore.language)}>
        {#each visibleNavItems as item}
          <button
            class:active={view === item.view}
            type="button"
            aria-current={view === item.view ? "page" : undefined}
            on:click={() => selectView(item.view)}
          >
            {#if item.view === "import"}
              <FileUp size={17} strokeWidth={1.8} aria-hidden="true" />
            {:else if item.view === "backup"}
              <DatabaseBackup size={17} strokeWidth={1.8} aria-hidden="true" />
            {:else if item.view === "fonts"}
              <Type size={17} strokeWidth={1.8} aria-hidden="true" />
            {:else if item.view === "profile"}
              <UserIcon size={17} strokeWidth={1.8} aria-hidden="true" />
            {:else}
              <Users size={17} strokeWidth={1.8} aria-hidden="true" />
            {/if}
            {t(item.labelKey, $preferencesStore.language)}
          </button>
        {/each}
        <button
          class="danger"
          type="button"
          on:click={() => {
            void logout();
          }}
        >
          <LogOut size={17} strokeWidth={1.8} aria-hidden="true" />
          {t("logout", $preferencesStore.language)}
        </button>
      </nav>

      <div class="tools-panel__content">
        {#if view === "import"}
          <ImportCenter showBack={false} onBack={() => selectView("profile")} onImported={handleImported} />
        {:else if view === "backup"}
          <BackupPanel />
        {:else if view === "fonts"}
          <FontPanel {user} />
        {:else if view === "profile"}
          <ProfilePanel {user} />
        {:else if view === "users"}
          <UserManagementPanel {user} />
        {/if}
      </div>
    </div>
  </div>
</div>
