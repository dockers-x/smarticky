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
  export let onSearch: () => void = () => {};

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
    <div class="sidebar__brand">
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

  <button
    class="sidebar__search"
    type="button"
    aria-label={t("searchNotes", $preferencesStore.language)}
    title={`${t("searchNotes", $preferencesStore.language)} (/)`}
    aria-keyshortcuts="/"
    on:click={onSearch}
  >
    <Search size={17} strokeWidth={1.8} aria-hidden="true" />
    <span class="sidebar__label">{t("searchNotes", $preferencesStore.language)}</span>
    {#if !$preferencesStore.sidebarCompact}<kbd aria-hidden="true">/</kbd>{/if}
  </button>

  <nav class="sidebar__nav" aria-label={$preferencesStore.language === "zh" ? "资料库" : "Library"}>
    {#each filters as filter (filter.id)}
      <button
        class:active={$notesStore.workspaceView === "notes" &&
          $notesStore.filter === filter.id &&
          !$notesStore.folderID &&
          !$notesStore.folderBrowserOpen &&
          !$notesStore.tagBrowserOpen &&
          $notesStore.searchFilters.tags.length === 0}
        type="button"
        aria-label={filter.label}
        aria-pressed={$notesStore.workspaceView === "notes" &&
          $notesStore.filter === filter.id &&
          !$notesStore.folderID &&
          !$notesStore.folderBrowserOpen &&
          !$notesStore.tagBrowserOpen &&
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
        aria-haspopup="dialog"
        aria-expanded={$notesStore.folderBrowserOpen}
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
        aria-haspopup="dialog"
        aria-expanded={$notesStore.tagBrowserOpen}
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
            aria-controls="sidebar-notebooks"
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
          <div id="sidebar-notebooks" class="sidebar-section__content sidebar-section__content--notebooks">
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
            aria-controls="sidebar-tags"
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
          <div id="sidebar-tags" class="sidebar-section__content sidebar-tag-list">
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
            <button class="sidebar-tag-list__all" type="button" on:click={onOpenTagBrowser}>
              <Search size={14} strokeWidth={1.8} aria-hidden="true" />
              <span>{$preferencesStore.language === "zh" ? "浏览全部标签" : "Browse all tags"}</span>
            </button>
          </div>
        {/if}
      </section>
    </div>
  {/if}

  <div class="sidebar__spacer"></div>
  <div
    class="sidebar__preferences"
    role="group"
    aria-label={t("settings", $preferencesStore.language)}
  >
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
    aria-haspopup="dialog"
    title={$preferencesStore.sidebarCompact ? t("settings", $preferencesStore.language) : undefined}
    on:click={onOpenSettings}
  >
    <Settings size={16} strokeWidth={1.8} aria-hidden="true" />
    <span class="sidebar__label">{t("settings", $preferencesStore.language)}</span>
  </button>
</aside>

<style>
  .sidebar {
    overflow-y: auto;
    overscroll-behavior: contain;
  }

  .sidebar__brand-row,
  .sidebar__nav,
  .sidebar__preferences,
  .sidebar__tool {
    flex-shrink: 0;
  }

  .sidebar__search {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 0 0 auto;
    min-width: 0;
    min-height: 40px;
    margin: 4px 0 12px;
    border: 1px solid var(--color-divider);
    border-radius: var(--radius-control);
    background: color-mix(in srgb, var(--color-card) 75%, var(--color-floor));
    color: var(--color-text-sub);
    padding: 0 10px;
    text-align: left;
    font-size: 12px;
  }

  .sidebar__search kbd {
    margin-inline-start: auto;
    border: 1px solid var(--color-divider);
    border-radius: 4px;
    padding: 0 5px;
    font-size: 10px;
    line-height: 18px;
  }

  .sidebar.compact .sidebar__search {
    width: 36px;
    min-width: 36px;
    justify-content: center;
    margin-inline: auto;
    padding: 0;
  }

  .sidebar__organize {
    min-height: 8rem;
  }

  .sidebar-tag-list__all {
    color: var(--color-ink-accent);
  }

  @media (pointer: coarse) {
    .sidebar__search {
      min-height: 44px;
    }

    .sidebar.compact .sidebar__search {
      width: 44px;
      min-width: 44px;
    }
  }

  @media (hover: hover) and (pointer: fine) {
    .sidebar__search:hover {
      border-color: var(--color-divider-em);
      color: var(--color-text);
    }
  }
</style>
