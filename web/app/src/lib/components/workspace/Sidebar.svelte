<script lang="ts">
  import {
    BookOpenText,
    Folder,
    Languages,
    Moon,
    PanelLeftClose,
    PanelLeftOpen,
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
    { id: "all" as NoteFilter, label: t("allNotes", $preferencesStore.language) },
    { id: "starred" as NoteFilter, label: t("starred", $preferencesStore.language) },
    { id: "trash" as NoteFilter, label: t("trash", $preferencesStore.language) },
  ];

  async function selectFilter(filter: NoteFilter): Promise<void> {
    foldersStore.select(null);
    await notesStore.setFilter(filter);
  }

  function openFolderBrowser(): void {
    notesStore.showFolderBrowser();
  }

  function handleFolderTabDragOver(event: DragEvent): void {
    if (!event.dataTransfer?.types.includes("application/x-smarticky-note-ids")) return;
    event.preventDefault();
    notesStore.showFolderBrowser();
    if (event.dataTransfer) event.dataTransfer.dropEffect = "move";
  }
</script>

<aside
  class:compact={$preferencesStore.sidebarCompact}
  class="sidebar"
  aria-label={t("noteList", $preferencesStore.language)}
>
  <div class="sidebar__brand-row">
    <div class="sidebar__brand" aria-label="Smarticky">
      <span class="sidebar__brand-short" aria-hidden="true">S</span>
      <span class="sidebar__label">Smarticky</span>
    </div>
    <button
      class="sidebar__collapse"
      type="button"
      aria-label={$preferencesStore.sidebarCompact
        ? t("expandSidebar", $preferencesStore.language)
        : t("collapseSidebar", $preferencesStore.language)}
      title={$preferencesStore.sidebarCompact
        ? t("expandSidebar", $preferencesStore.language)
        : t("collapseSidebar", $preferencesStore.language)}
      on:click={() => preferencesStore.toggleSidebarCompact()}
    >
      {#if $preferencesStore.sidebarCompact}
        <PanelLeftOpen size={17} strokeWidth={1.8} aria-hidden="true" />
      {:else}
        <PanelLeftClose size={17} strokeWidth={1.8} aria-hidden="true" />
      {/if}
    </button>
  </div>
  <nav class="sidebar__nav">
    {#each filters as filter}
      <button
        class:active={$notesStore.filter === filter.id &&
          !$notesStore.folderID &&
          !$notesStore.folderBrowserOpen}
        type="button"
        aria-label={filter.label}
        aria-pressed={$notesStore.filter === filter.id &&
          !$notesStore.folderID &&
          !$notesStore.folderBrowserOpen}
        title={$preferencesStore.sidebarCompact ? filter.label : undefined}
        on:click={() => void selectFilter(filter.id)}
      >
        {#if filter.id === "all"}
          <BookOpenText size={17} strokeWidth={1.8} aria-hidden="true" />
        {:else if filter.id === "starred"}
          <Star size={17} strokeWidth={1.8} aria-hidden="true" />
        {:else}
          <Trash2 size={17} strokeWidth={1.8} aria-hidden="true" />
        {/if}
        <span class="sidebar__label">{filter.label}</span>
      </button>
      {#if filter.id === "all"}
        <button
          class:active={$notesStore.folderBrowserOpen ||
            ($notesStore.filter === "all" && Boolean($notesStore.folderID))}
          type="button"
          aria-label={t("notebookGroups", $preferencesStore.language)}
          aria-pressed={$notesStore.folderBrowserOpen ||
            ($notesStore.filter === "all" && Boolean($notesStore.folderID))}
          title={$preferencesStore.sidebarCompact
            ? t("notebookGroups", $preferencesStore.language)
            : undefined}
          on:dragover={handleFolderTabDragOver}
          on:click={openFolderBrowser}
        >
          <Folder size={17} strokeWidth={1.8} aria-hidden="true" />
          <span class="sidebar__label">{t("notebookGroups", $preferencesStore.language)}</span>
        </button>
      {/if}
    {/each}
  </nav>

  <div class="sidebar__spacer"></div>
  <div class="sidebar__preferences" aria-label={t("settings", $preferencesStore.language)}>
    <button
      class="sidebar__icon-tool"
      type="button"
      aria-label={$preferencesStore.theme === "dark"
        ? t("lightTheme", $preferencesStore.language)
        : t("darkTheme", $preferencesStore.language)}
      title={$preferencesStore.theme === "dark"
        ? t("lightTheme", $preferencesStore.language)
        : t("darkTheme", $preferencesStore.language)}
      on:click={() => preferencesStore.toggleTheme()}
    >
      {#if $preferencesStore.theme === "dark"}
        <Sun size={17} strokeWidth={1.8} />
      {:else}
        <Moon size={17} strokeWidth={1.8} />
      {/if}
    </button>
    <button
      type="button"
      aria-label={t("language", $preferencesStore.language)}
      title={$preferencesStore.sidebarCompact ? t("language", $preferencesStore.language) : undefined}
      on:click={() => preferencesStore.toggleLanguage()}
    >
      <Languages size={15} strokeWidth={1.8} aria-hidden="true" />
      <span class="sidebar__label">{$preferencesStore.language === "zh" ? "EN" : "中文"}</span>
    </button>
  </div>
  <button
    class="sidebar__tool"
    type="button"
    aria-label={t("settings", $preferencesStore.language)}
    aria-expanded={settingsOpen}
    title={$preferencesStore.sidebarCompact ? t("settings", $preferencesStore.language) : undefined}
    on:click={() => (settingsOpen = !settingsOpen)}
  >
    <Settings size={16} strokeWidth={1.8} aria-hidden="true" />
    <span class="sidebar__label">{t("settings", $preferencesStore.language)}</span>
  </button>
  {#if settingsOpen}
    <ToolsPanel user={$authStore.user} onClose={() => (settingsOpen = false)} />
  {/if}
</aside>
