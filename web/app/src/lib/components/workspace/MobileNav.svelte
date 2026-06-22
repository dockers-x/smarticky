<script lang="ts">
  import {
    BookOpenText,
    Folder,
    Languages,
    Moon,
    Network,
    Settings,
    Star,
    Sun,
    Trash2,
  } from "@lucide/svelte";
  import ToolsPanel from "../settings/ToolsPanel.svelte";
  import { authStore } from "../../stores/auth";
  import { foldersStore } from "../../stores/folders";
  import { notesStore, type NoteFilter } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";

  let settingsOpen = false;

  $: filters = [
    { id: "all" as NoteFilter, label: t("allNotes", $preferencesStore.language), icon: BookOpenText },
    { id: "starred" as NoteFilter, label: t("starred", $preferencesStore.language), icon: Star },
    { id: "trash" as NoteFilter, label: t("trash", $preferencesStore.language), icon: Trash2 },
  ];

  async function selectFilter(filter: NoteFilter): Promise<void> {
    foldersStore.select(null);
    await notesStore.setFilter(filter);
  }

  async function selectIndex(): Promise<void> {
    foldersStore.select(null);
    await notesStore.setWorkspaceView("index");
  }

  function openFolderBrowser(): void {
    notesStore.showFolderBrowser();
  }
</script>

<nav
  class:editor-open={Boolean($notesStore.selected)}
  class="mobile-nav"
  aria-label={t("noteList", $preferencesStore.language)}
>
  <div class="mobile-nav__row mobile-nav__row--primary">
    {#each filters as filter (filter.id)}
      <button
        class:active={$notesStore.workspaceView === "notes" &&
          $notesStore.filter === filter.id &&
          !$notesStore.folderID &&
          !$notesStore.folderBrowserOpen}
        type="button"
        aria-label={filter.label}
        aria-pressed={$notesStore.workspaceView === "notes" &&
          $notesStore.filter === filter.id &&
          !$notesStore.folderID &&
          !$notesStore.folderBrowserOpen}
        on:click={() => void selectFilter(filter.id)}
      >
        <svelte:component this={filter.icon} size={18} strokeWidth={1.8} aria-hidden="true" />
        <span>{filter.label}</span>
      </button>
    {/each}
    <button
      class:active={$notesStore.workspaceView === "index"}
      type="button"
      aria-label={t("index", $preferencesStore.language)}
      aria-pressed={$notesStore.workspaceView === "index"}
      on:click={() => void selectIndex()}
    >
      <Network size={18} strokeWidth={1.8} aria-hidden="true" />
      <span>{t("index", $preferencesStore.language)}</span>
    </button>
    <button
      class:active={$notesStore.workspaceView === "notes" &&
        ($notesStore.folderBrowserOpen ||
          ($notesStore.filter === "all" && Boolean($notesStore.folderID)))}
      type="button"
      aria-label={t("notebookGroups", $preferencesStore.language)}
      aria-pressed={$notesStore.workspaceView === "notes" &&
        ($notesStore.folderBrowserOpen ||
          ($notesStore.filter === "all" && Boolean($notesStore.folderID)))}
      on:click={openFolderBrowser}
    >
      <Folder size={18} strokeWidth={1.8} aria-hidden="true" />
      <span>{t("notebookGroups", $preferencesStore.language)}</span>
    </button>
  </div>

  <div class="mobile-nav__row mobile-nav__row--utility">
    <button
      type="button"
      aria-label={$preferencesStore.theme === "dark"
        ? t("lightTheme", $preferencesStore.language)
        : t("darkTheme", $preferencesStore.language)}
      on:click={() => preferencesStore.toggleTheme()}
    >
      {#if $preferencesStore.theme === "dark"}
        <Sun size={17} strokeWidth={1.8} aria-hidden="true" />
        <span>{t("lightTheme", $preferencesStore.language)}</span>
      {:else}
        <Moon size={17} strokeWidth={1.8} aria-hidden="true" />
        <span>{t("darkTheme", $preferencesStore.language)}</span>
      {/if}
    </button>
    <button
      type="button"
      aria-label={t("language", $preferencesStore.language)}
      on:click={() => preferencesStore.toggleLanguage()}
    >
      <Languages size={17} strokeWidth={1.8} aria-hidden="true" />
      <span>{$preferencesStore.language === "zh" ? "EN" : "中文"}</span>
    </button>
    <button
      type="button"
      aria-label={t("settings", $preferencesStore.language)}
      aria-expanded={settingsOpen}
      on:click={() => (settingsOpen = true)}
    >
      <Settings size={17} strokeWidth={1.8} aria-hidden="true" />
      <span>{t("settings", $preferencesStore.language)}</span>
    </button>
  </div>
</nav>

{#if settingsOpen}
  <ToolsPanel user={$authStore.user} onClose={() => (settingsOpen = false)} />
{/if}
