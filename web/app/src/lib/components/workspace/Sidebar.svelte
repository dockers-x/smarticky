<script lang="ts">
  import { onMount } from "svelte";
  import {
    BookOpenText,
    ChevronDown,
    ChevronRight,
    Folder,
    Languages,
    Moon,
    Network,
    PanelLeftClose,
    PanelLeftOpen,
    Plus,
    Search,
    Settings,
    Star,
    Sun,
    Tag,
    Trash2,
  } from "@lucide/svelte";
  import { inputDialog, notify } from "../../stores/dialogs";
  import { foldersStore } from "../../stores/folders";
  import { notesStore, type NoteFilter } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";
  import { tagsStore } from "../../stores/tags";
  import FolderBrowserPane from "./FolderBrowserPane.svelte";

  export let settingsOpen = false;
  export let onOpenSettings: () => void = () => {};
  export let onOpenFolderBrowser: () => void = () => {};
  export let onOpenTagBrowser: () => void = () => {};

  let notebooksExpanded = true;
  let tagsExpanded = false;

  $: filters = [
    { id: "all" as NoteFilter, label: t("allNotes", $preferencesStore.language) },
    { id: "starred" as NoteFilter, label: t("starred", $preferencesStore.language) },
    { id: "trash" as NoteFilter, label: t("trash", $preferencesStore.language) },
  ];
  $: visibleTags = [...$tagsStore]
    .sort((left, right) => {
      const countDifference = (right.note_count ?? 0) - (left.note_count ?? 0);
      return countDifference || left.name.localeCompare(right.name);
    })
    .slice(0, 6);

  onMount(() => {
    void tagsStore.load();
  });

  async function selectFilter(filter: NoteFilter): Promise<void> {
    foldersStore.select(null);
    await notesStore.setFilter(filter);
  }

  async function selectIndex(): Promise<void> {
    foldersStore.select(null);
    await notesStore.setWorkspaceView("index");
  }

  async function selectTag(tagName: string): Promise<void> {
    await notesStore.setTagFilter([tagName]);
  }

  async function createRootNotebook(): Promise<void> {
    const name = await inputDialog({
      title: t("newNotebookGroup", $preferencesStore.language),
      label: t("folderName", $preferencesStore.language),
      confirmLabel: t("add", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
      requiredMessage: t("folderNameRequired", $preferencesStore.language),
    });
    if (!name) return;

    try {
      await foldersStore.create({ name, parent_id: null });
      notebooksExpanded = true;
      notify(t("folderCreated", $preferencesStore.language), "success");
    } catch {
      notify(t("folderCreateFailed", $preferencesStore.language), "error");
    }
  }

  function handleFolderTabDragOver(event: DragEvent): void {
    if (!event.dataTransfer?.types.includes("application/x-smarticky-note-ids")) return;
    event.preventDefault();
    onOpenFolderBrowser();
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
    {#each filters as filter (filter.id)}
      <button
        class:active={$notesStore.workspaceView === "notes" &&
          $notesStore.filter === filter.id &&
          !$notesStore.folderID &&
          $notesStore.searchFilters.tags.length === 0}
        type="button"
        aria-label={filter.label}
        aria-pressed={$notesStore.workspaceView === "notes" &&
          $notesStore.filter === filter.id &&
          !$notesStore.folderID &&
          $notesStore.searchFilters.tags.length === 0}
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
          class:active={$notesStore.workspaceView === "index"}
          type="button"
          aria-label={t("index", $preferencesStore.language)}
          aria-pressed={$notesStore.workspaceView === "index"}
          title={$preferencesStore.sidebarCompact
            ? t("index", $preferencesStore.language)
            : undefined}
          on:click={() => void selectIndex()}
        >
          <Network size={17} strokeWidth={1.8} aria-hidden="true" />
          <span class="sidebar__label">{t("index", $preferencesStore.language)}</span>
        </button>
      {/if}
    {/each}
  </nav>

  {#if $preferencesStore.sidebarCompact}
    <div class="sidebar__compact-destinations">
      <button
        class:active={$notesStore.workspaceView === "notes" &&
          ($notesStore.folderBrowserOpen ||
            ($notesStore.filter === "all" && Boolean($notesStore.folderID)))}
        type="button"
        aria-label={t("notebookGroups", $preferencesStore.language)}
        title={t("notebookGroups", $preferencesStore.language)}
        on:dragover={handleFolderTabDragOver}
        on:click={onOpenFolderBrowser}
      >
        <Folder size={17} strokeWidth={1.8} aria-hidden="true" />
      </button>
      <button
        class:active={$notesStore.workspaceView === "notes" &&
          ($notesStore.tagBrowserOpen || $notesStore.searchFilters.tags.length > 0)}
        type="button"
        aria-label={t("tags", $preferencesStore.language)}
        title={t("tags", $preferencesStore.language)}
        on:click={onOpenTagBrowser}
      >
        <Tag size={17} strokeWidth={1.8} aria-hidden="true" />
      </button>
    </div>
  {:else}
    <div class="sidebar__organize">
      <section class:expanded={notebooksExpanded} class="sidebar-section sidebar-section--notebooks">
        <header class="sidebar-section__header">
          <button
            class="sidebar-section__toggle"
            type="button"
            aria-expanded={notebooksExpanded}
            on:click={() => (notebooksExpanded = !notebooksExpanded)}
          >
            {#if notebooksExpanded}
              <ChevronDown size={14} strokeWidth={2} aria-hidden="true" />
            {:else}
              <ChevronRight size={14} strokeWidth={2} aria-hidden="true" />
            {/if}
            <span>{t("notebookGroups", $preferencesStore.language)}</span>
          </button>
          <div class="sidebar-section__actions">
            <button
              type="button"
              aria-label={t("searchNotebookGroups", $preferencesStore.language)}
              title={t("searchNotebookGroups", $preferencesStore.language)}
              on:click={onOpenFolderBrowser}
            >
              <Search size={14} strokeWidth={2} aria-hidden="true" />
            </button>
            <button
              type="button"
              aria-label={t("newNotebookGroup", $preferencesStore.language)}
              title={t("newNotebookGroup", $preferencesStore.language)}
              on:click={() => void createRootNotebook()}
            >
              <Plus size={14} strokeWidth={2} aria-hidden="true" />
            </button>
          </div>
        </header>
        {#if notebooksExpanded}
          <div class="sidebar-section__content sidebar-section__content--notebooks">
            <FolderBrowserPane variant="sidebar" />
          </div>
        {/if}
      </section>

      <section class:expanded={tagsExpanded} class="sidebar-section sidebar-section--tags">
        <header class="sidebar-section__header">
          <button
            class="sidebar-section__toggle"
            type="button"
            aria-expanded={tagsExpanded}
            on:click={() => (tagsExpanded = !tagsExpanded)}
          >
            {#if tagsExpanded}
              <ChevronDown size={14} strokeWidth={2} aria-hidden="true" />
            {:else}
              <ChevronRight size={14} strokeWidth={2} aria-hidden="true" />
            {/if}
            <span>{t("tags", $preferencesStore.language)}</span>
          </button>
          <div class="sidebar-section__actions">
            <button
              type="button"
              aria-label={t("searchTags", $preferencesStore.language)}
              title={t("searchTags", $preferencesStore.language)}
              on:click={onOpenTagBrowser}
            >
              <Search size={14} strokeWidth={2} aria-hidden="true" />
            </button>
          </div>
        </header>
        {#if tagsExpanded}
          <div class="sidebar-section__content sidebar-tag-list">
            {#if visibleTags.length === 0}
              <p>{t("noTags", $preferencesStore.language)}</p>
            {:else}
              {#each visibleTags as tag (tag.id)}
                <button
                  class:active={$notesStore.searchFilters.tags.includes(tag.name)}
                  type="button"
                  aria-pressed={$notesStore.searchFilters.tags.includes(tag.name)}
                  title={tag.name}
                  on:click={() => void selectTag(tag.name)}
                >
                  <Tag size={14} strokeWidth={1.8} aria-hidden="true" />
                  <span>{tag.name}</span>
                  <small>{tag.note_count ?? 0}</small>
                </button>
              {/each}
            {/if}
          </div>
        {/if}
      </section>
    </div>
  {/if}

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
    on:click={onOpenSettings}
  >
    <Settings size={16} strokeWidth={1.8} aria-hidden="true" />
    <span class="sidebar__label">{t("settings", $preferencesStore.language)}</span>
  </button>
</aside>
