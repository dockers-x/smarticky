<script lang="ts">
  import { onMount, tick } from "svelte";
  import {
    BookOpenText,
    Ellipsis,
    Folder,
    Languages,
    Moon,
    Network,
    Settings,
    Star,
    Sun,
    Tag,
    Trash2,
    X,
  } from "@lucide/svelte";
  import { dialog } from "../../actions/dialog";
  import { foldersStore } from "../../stores/folders";
  import { notesStore, type NoteFilter } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";

  export let settingsOpen = false;
  export let onOpenSettings: () => void = () => {};
  export let onOpenFolderBrowser: () => void = () => {};
  export let onOpenTagBrowser: () => void = () => {};

  let moreOpen = false;

  $: filters = [
    { id: "all" as NoteFilter, label: t("allNotes", $preferencesStore.language), icon: BookOpenText },
    { id: "starred" as NoteFilter, label: t("starred", $preferencesStore.language), icon: Star },
  ];
  $: moreLabel = $preferencesStore.language === "zh" ? "更多" : "More";
  $: if ($notesStore.selected) moreOpen = false;

  onMount(() => {
    const closeOnDesktop = () => {
      if (window.innerWidth > 960) moreOpen = false;
    };
    window.addEventListener("resize", closeOnDesktop);
    return () => window.removeEventListener("resize", closeOnDesktop);
  });

  async function selectFilter(filter: NoteFilter): Promise<void> {
    foldersStore.select(null);
    await notesStore.setFilter(filter);
  }

  async function selectIndex(): Promise<void> {
    foldersStore.select(null);
    notesStore.clearSelection();
    await notesStore.setWorkspaceView("index");
  }

  async function runMoreAction(action: () => void | Promise<void>): Promise<void> {
    moreOpen = false;
    await tick();
    await action();
  }
</script>

<nav
  class:editor-open={Boolean($notesStore.selected)}
  class="mobile-nav"
  aria-label={$preferencesStore.language === "zh" ? "主导航" : "Main navigation"}
>
  <div class="mobile-nav__row mobile-nav__row--primary">
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
        on:click={() => void selectFilter(filter.id)}
      >
        <svelte:component this={filter.icon} size={18} strokeWidth={1.8} aria-hidden="true" />
        <span>{filter.label}</span>
      </button>
    {/each}
    <button
      class:active={$notesStore.workspaceView === "notes" &&
        ($notesStore.folderBrowserOpen ||
          ($notesStore.filter === "all" && Boolean($notesStore.folderID)))}
      type="button"
      aria-label={t("notebookGroups", $preferencesStore.language)}
      aria-haspopup="dialog"
      aria-expanded={$notesStore.folderBrowserOpen}
      on:click={onOpenFolderBrowser}
    >
      <Folder size={18} strokeWidth={1.8} aria-hidden="true" />
      <span>{$preferencesStore.language === "zh" ? "笔记本" : "Notebooks"}</span>
    </button>
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
      class:active={moreOpen || settingsOpen || ($notesStore.workspaceView === "notes" &&
        ($notesStore.tagBrowserOpen || $notesStore.searchFilters.tags.length > 0 ||
          $notesStore.filter === "trash"))}
      type="button"
      aria-label={moreLabel}
      aria-haspopup="dialog"
      aria-expanded={moreOpen}
      aria-controls="mobile-more-dialog"
      on:click={() => (moreOpen = true)}
    >
      <Ellipsis size={18} strokeWidth={1.8} aria-hidden="true" />
      <span>{moreLabel}</span>
    </button>
  </div>
</nav>

{#if moreOpen}
  <div class="mobile-more-backdrop" role="presentation">
    <div
      id="mobile-more-dialog"
      class="mobile-more"
      role="dialog"
      aria-modal="true"
      aria-labelledby="mobile-more-title"
      tabindex="-1"
      use:dialog={{ onDismiss: () => (moreOpen = false) }}
    >
      <header class="mobile-more__header">
        <h2 id="mobile-more-title">{moreLabel}</h2>
        <button
          type="button"
          aria-label={t("cancel", $preferencesStore.language)}
          on:click={() => (moreOpen = false)}
        >
          <X size={19} strokeWidth={1.8} aria-hidden="true" />
        </button>
      </header>
      <div class="mobile-more__group">
        <button type="button" on:click={() => void runMoreAction(onOpenTagBrowser)}>
          <Tag size={19} strokeWidth={1.8} aria-hidden="true" />
          <span>{t("tags", $preferencesStore.language)}</span>
        </button>
        <button type="button" on:click={() => void runMoreAction(() => selectFilter("trash"))}>
          <Trash2 size={19} strokeWidth={1.8} aria-hidden="true" />
          <span>{t("trash", $preferencesStore.language)}</span>
        </button>
      </div>
      <div class="mobile-more__group">
        <button type="button" on:click={() => preferencesStore.toggleTheme()}>
          {#if $preferencesStore.theme === "dark"}
            <Sun size={19} strokeWidth={1.8} aria-hidden="true" />
            <span>{t("lightTheme", $preferencesStore.language)}</span>
          {:else}
            <Moon size={19} strokeWidth={1.8} aria-hidden="true" />
            <span>{t("darkTheme", $preferencesStore.language)}</span>
          {/if}
        </button>
        <button type="button" on:click={() => preferencesStore.toggleLanguage()}>
          <Languages size={19} strokeWidth={1.8} aria-hidden="true" />
          <span>{t("language", $preferencesStore.language)}</span>
          <small>{$preferencesStore.language === "zh" ? "English" : "简体中文"}</small>
        </button>
        <button type="button" on:click={() => void runMoreAction(onOpenSettings)}>
          <Settings size={19} strokeWidth={1.8} aria-hidden="true" />
          <span>{t("settings", $preferencesStore.language)}</span>
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  @media (max-width: 960px) {
    .mobile-nav {
      grid-column: 1;
      grid-row: 1;
      padding-inline: max(8px, env(safe-area-inset-left)) max(8px, env(safe-area-inset-right));
    }

    .mobile-nav__row {
      display: grid;
      grid-template-columns: repeat(5, minmax(0, 1fr));
      gap: 4px;
      overflow: visible;
    }

    .mobile-nav__row > button {
      min-width: 0;
      min-height: 52px;
      flex-direction: column;
      gap: 4px;
      padding: 6px 2px;
      font-size: 11px;
      line-height: 1.2;
    }

    .mobile-nav__row > button span {
      max-width: 100%;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  .mobile-more-backdrop {
    position: absolute;
    inset: 0;
    z-index: 75;
    display: flex;
    align-items: flex-end;
    justify-content: center;
    background: var(--color-backdrop);
    padding: max(12px, env(safe-area-inset-top)) max(12px, env(safe-area-inset-right))
      max(12px, env(safe-area-inset-bottom)) max(12px, env(safe-area-inset-left));
  }

  .mobile-more {
    width: min(100%, 420px);
    max-height: 100%;
    overflow-y: auto;
    overscroll-behavior: contain;
    border: 1px solid var(--color-divider);
    border-radius: 18px;
    background: var(--color-card);
    padding: 8px 12px 12px;
    box-shadow: var(--shadow-panel);
  }

  .mobile-more__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .mobile-more__header h2 {
    margin: 0;
    padding-inline: 8px;
    font-size: 16px;
    font-weight: 650;
    letter-spacing: -0.02em;
  }

  .mobile-more button {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    min-height: 48px;
    border: 0;
    border-radius: var(--radius-control);
    background: transparent;
    padding: 10px;
    color: var(--color-text-secondary);
    text-align: left;
    font-size: 14px;
  }

  .mobile-more button :global(svg) {
    flex: 0 0 auto;
  }

  .mobile-more button small {
    margin-inline-start: auto;
    color: var(--color-text-sub);
    font-size: 12px;
  }

  .mobile-more__header button {
    width: 44px;
    min-height: 44px;
    justify-content: center;
    padding: 0;
  }

  .mobile-more__group {
    display: grid;
    padding-block: 8px;
  }

  .mobile-more__group + .mobile-more__group {
    border-top: 1px solid var(--color-divider);
  }

  @media (hover: hover) and (pointer: fine) {
    .mobile-more button:hover {
      background: var(--color-surface-secondary);
    }
  }
</style>
