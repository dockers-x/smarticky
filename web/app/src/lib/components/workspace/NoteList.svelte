<script lang="ts">
  import { onMount } from "svelte";
  import {
    ArrowLeft,
    CheckSquare,
    ChevronRight,
    Folder,
    FolderOpen,
    FolderInput,
    Plus,
    SlidersHorizontal,
    Square,
    Trash2,
  } from "@lucide/svelte";
  import type { Folder as FolderType, Note } from "../../api/types";
  import { confirmDialog, notify } from "../../stores/dialogs";
  import { foldersStore } from "../../stores/folders";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";
  import { tagsStore } from "../../stores/tags";
  import EmptyState from "./EmptyState.svelte";
  import FolderBrowserPane from "./FolderBrowserPane.svelte";
  import NoteCard from "./NoteCard.svelte";

  interface NoteGroup {
    label: string;
    notes: Note[];
  }

  let filterPanelOpen = false;
  let selectedNoteIDs: string[] = [];

  onMount(() => {
    void tagsStore.load();
  });

  $: filters = [
    { id: "all" as const, label: t("allNotes", $preferencesStore.language) },
    { id: "starred" as const, label: t("starred", $preferencesStore.language) },
    { id: "trash" as const, label: t("trash", $preferencesStore.language) },
  ];
  $: activeFolder =
    $notesStore.folderID && $notesStore.folderID !== "unfiled"
      ? $foldersStore.folders.find((folder) => folder.id === $notesStore.folderID)
      : null;
  $: folderByID = new Map($foldersStore.folders.map((folder) => [folder.id, folder]));
  $: activeFolderPath = activeFolder
    ? buildFolderPath(activeFolder, folderByID)
    : [];
  $: childFolders = activeFolder
    ? sortFolders(
        $foldersStore.folders.filter(
          (folder) => folder.parent_id === activeFolder?.id,
        ),
      )
    : [];
  $: viewTitle =
    $notesStore.filter === "trash"
      ? t("trash", $preferencesStore.language)
      : $notesStore.filter === "starred"
        ? t("starred", $preferencesStore.language)
        : $notesStore.folderID === "unfiled"
          ? t("unfiledNotes", $preferencesStore.language)
          : activeFolder?.name ?? t("allNotes", $preferencesStore.language);
  $: folderViewActive = $notesStore.filter === "all" && Boolean($notesStore.folderID);
  $: starredFolders = $foldersStore.folders.filter((folder) => folder.is_starred);
  $: selectedCount = selectedNoteIDs.length;
  $: advancedFilterCount =
    ($notesStore.searchFilters.title.trim() ? 1 : 0) +
    $notesStore.searchFilters.tags.length +
    ($notesStore.searchFilters.createdFrom ? 1 : 0) +
    ($notesStore.searchFilters.createdTo ? 1 : 0) +
    ($notesStore.searchFilters.updatedFrom ? 1 : 0) +
    ($notesStore.searchFilters.updatedTo ? 1 : 0);
  $: visibleNoteIDs = new Set($notesStore.notes.map((note) => note.id));
  $: {
    const nextSelectedNoteIDs = selectedNoteIDs.filter((id) =>
      visibleNoteIDs.has(id),
    );
    if (nextSelectedNoteIDs.length !== selectedNoteIDs.length) {
      selectedNoteIDs = nextSelectedNoteIDs;
    }
  }

  function dateKey(date: Date, timeZone: string): string {
    const parts = new Intl.DateTimeFormat("en-CA", {
      timeZone,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).formatToParts(date);
    const value = (type: string) =>
      parts.find((part) => part.type === type)?.value ?? "";
    return `${value("year")}-${value("month")}-${value("day")}`;
  }

  function groupLabel(date: Date, language: "zh" | "en", timeZone: string): string {
    const today = new Date();
    const yesterday = new Date(Date.now() - 24 * 60 * 60 * 1000);

    const noteKey = dateKey(date, timeZone);
    if (noteKey === dateKey(today, timeZone)) return t("today", language);
    if (noteKey === dateKey(yesterday, timeZone)) return t("yesterday", language);

    const noteYear = noteKey.slice(0, 4);
    const todayYear = dateKey(today, timeZone).slice(0, 4);

    return new Intl.DateTimeFormat(language === "zh" ? "zh-CN" : "en-US", {
      month: "long",
      day: "numeric",
      year: noteYear === todayYear ? undefined : "numeric",
      timeZone,
    }).format(date);
  }

  $: groupedNotes = $notesStore.notes.reduce<NoteGroup[]>((groups, note) => {
    const label = groupLabel(
      new Date(note.updated_at),
      $preferencesStore.language,
      $preferencesStore.timeZone,
    );
    const group = groups.find((item) => item.label === label);
    if (group) {
      group.notes.push(note);
    } else {
      groups.push({ label, notes: [note] });
    }
    return groups;
  }, []);

  function sortFolders(folders: FolderType[]): FolderType[] {
    return [...folders].sort((left, right) => {
      if (left.sort_order !== right.sort_order) {
        return left.sort_order - right.sort_order;
      }
      return left.name.localeCompare(right.name);
    });
  }

  function buildFolderPath(
    folder: FolderType,
    foldersByID: Map<string, FolderType>,
  ): FolderType[] {
    const path: FolderType[] = [];
    const visited = new Set<string>();
    let current: FolderType | undefined = folder;

    while (current && !visited.has(current.id)) {
      path.unshift(current);
      visited.add(current.id);
      current = current.parent_id ? foldersByID.get(current.parent_id) : undefined;
    }

    return path;
  }

  function toggleSelected(noteID: string): void {
    selectedNoteIDs = selectedNoteIDs.includes(noteID)
      ? selectedNoteIDs.filter((id) => id !== noteID)
      : [...selectedNoteIDs, noteID];
  }

  function clearSelection(): void {
    selectedNoteIDs = [];
  }

  function selectAllVisible(): void {
    selectedNoteIDs = $notesStore.notes.map((note) => note.id);
  }

  async function toggleSearchTag(tagName: string): Promise<void> {
    const tags = $notesStore.searchFilters.tags.includes(tagName)
      ? $notesStore.searchFilters.tags.filter((name) => name !== tagName)
      : [...$notesStore.searchFilters.tags, tagName];
    await notesStore.setSearchFilters({ tags });
  }

  async function selectFolder(folderID: string): Promise<void> {
    foldersStore.select(folderID);
    await notesStore.setFolder(folderID);
  }

  function returnToNotebookGroups(): void {
    notesStore.showFolderBrowser();
  }

  function openMovePane(): void {
    if (selectedNoteIDs.length === 0) return;
    notesStore.showFolderBrowser();
  }

  async function deleteSelected(): Promise<void> {
    if (selectedNoteIDs.length === 0) return;

    const permanent = $notesStore.filter === "trash";
    const confirmed = await confirmDialog({
      title: permanent
        ? t("deleteSelectedNotes", $preferencesStore.language)
        : t("trashSelectedNotes", $preferencesStore.language),
      message: permanent
        ? t("deleteSelectedNotesMessage", $preferencesStore.language)
        : t("trashSelectedNotesMessage", $preferencesStore.language),
      confirmLabel: permanent
        ? t("deleteForever", $preferencesStore.language)
        : t("trashNote", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
    });
    if (!confirmed) return;

    const ids = [...selectedNoteIDs];
    try {
      if (permanent) {
        await Promise.all(ids.map((id) => notesStore.deletePermanent(id)));
        notify(t("deletedNotes", $preferencesStore.language), "success");
      } else {
        await Promise.all(ids.map((id) => notesStore.updateNote(id, { is_deleted: true })));
        notify(t("trashedNotes", $preferencesStore.language), "success");
      }
      clearSelection();
      await Promise.all([notesStore.load(), foldersStore.load()]);
    } catch {
      notify(
        permanent
          ? t("deleteForeverFailed", $preferencesStore.language)
          : t("trashFailed", $preferencesStore.language),
        "error",
      );
    }
  }

  async function toggleNoteStar(note: Note): Promise<void> {
    try {
      await notesStore.updateNote(note.id, { is_starred: !note.is_starred });
      await notesStore.load();
    } catch {
      notify(t("updateStarFailed", $preferencesStore.language), "error");
    }
  }

  async function deleteNoteFromList(note: Note): Promise<void> {
    if (note.is_deleted) {
      const confirmed = await confirmDialog({
        title: t("deleteForever", $preferencesStore.language),
        message: t("deleteForeverMessage", $preferencesStore.language),
        confirmLabel: t("deleteForever", $preferencesStore.language),
        cancelLabel: t("cancel", $preferencesStore.language),
      });
      if (!confirmed) return;

      try {
        await notesStore.deletePermanent(note.id);
        selectedNoteIDs = selectedNoteIDs.filter((id) => id !== note.id);
        notify(t("deletedNote", $preferencesStore.language), "success");
      } catch {
        notify(t("deleteForeverFailed", $preferencesStore.language), "error");
      }
      return;
    }

    const confirmed = await confirmDialog({
      title: t("trashNote", $preferencesStore.language),
      message: t("trashNoteMessage", $preferencesStore.language),
      confirmLabel: t("trashNote", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
    });
    if (!confirmed) return;

    try {
      await notesStore.updateNote(note.id, { is_deleted: true });
      selectedNoteIDs = selectedNoteIDs.filter((id) => id !== note.id);
      await Promise.all([notesStore.load(), foldersStore.load()]);
      notify(t("trashedNote", $preferencesStore.language), "success");
    } catch {
      notify(t("trashFailed", $preferencesStore.language), "error");
    }
  }

  async function emptyTrash(): Promise<void> {
    const confirmed = await confirmDialog({
      title: t("emptyTrash", $preferencesStore.language),
      message: t("emptyTrashMessage", $preferencesStore.language),
      confirmLabel: t("emptyTrash", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
    });
    if (!confirmed) return;

    try {
      await notesStore.emptyTrash();
      clearSelection();
      await foldersStore.load();
      notify(t("emptiedTrash", $preferencesStore.language), "success");
    } catch {
      notify(t("emptyTrashFailed", $preferencesStore.language), "error");
    }
  }
</script>

<section
  class:editor-open={Boolean($notesStore.selected)}
  class="note-list-pane"
  aria-label={t("noteList", $preferencesStore.language)}
>
  {#if $notesStore.folderBrowserOpen}
    <FolderBrowserPane
      {selectedNoteIDs}
      onSelectionMoved={clearSelection}
    />
  {:else}
  <div class="note-list-titlebar">
    <div class="note-list-titlebar__heading">
      {#if folderViewActive}
        <button
          class="note-list-titlebar__back"
          type="button"
          aria-label={t("backToNotebookGroups", $preferencesStore.language)}
          title={t("backToNotebookGroups", $preferencesStore.language)}
          on:click={returnToNotebookGroups}
        >
          <ArrowLeft size={15} strokeWidth={2} aria-hidden="true" />
          <span>{t("notebookGroups", $preferencesStore.language)}</span>
        </button>
      {/if}
      <div class="note-list-titlebar__copy">
        <h1 title={viewTitle}>{viewTitle}</h1>
        <span>{selectedCount > 0 ? `${selectedCount} ${t("selectedNotes", $preferencesStore.language)}` : `${$notesStore.notes.length} ${t("notes", $preferencesStore.language)}`}</span>
        {#if activeFolderPath.length > 0}
          <nav
            class="note-list-breadcrumb"
            aria-label={t("notebookPath", $preferencesStore.language)}
          >
            <button type="button" on:click={returnToNotebookGroups}>
              {t("notebookGroups", $preferencesStore.language)}
            </button>
            {#each activeFolderPath as folder, index (folder.id)}
              <ChevronRight size={13} strokeWidth={2} aria-hidden="true" />
              {#if index < activeFolderPath.length - 1}
                <button
                  type="button"
                  title={folder.name}
                  on:click={() => void selectFolder(folder.id)}
                >
                  {folder.name}
                </button>
              {:else}
                <span title={folder.name}>{folder.name}</span>
              {/if}
            {/each}
          </nav>
        {/if}
      </div>
    </div>
    {#if $notesStore.filter !== "trash"}
      <button
        class="note-list-titlebar__new"
        type="button"
        aria-label={t("newNote", $preferencesStore.language)}
        on:click={() => notesStore.create()}
      >
        <Plus size={18} strokeWidth={2} aria-hidden="true" />
      </button>
    {/if}
  </div>

  <div class="note-list-toolbar">
    <input
      type="search"
      aria-label={t("searchNotes", $preferencesStore.language)}
      placeholder={t("searchNotes", $preferencesStore.language)}
      value={$notesStore.search}
      on:input={(event) => notesStore.setSearch(event.currentTarget.value)}
    />
    <div class="note-list-toolbar__actions">
      {#if $notesStore.filter === "trash" && $notesStore.notes.length > 0}
        <button class="note-list-danger-tool" type="button" on:click={emptyTrash}>
          {t("emptyTrash", $preferencesStore.language)}
        </button>
      {/if}
      <button
        class:active={filterPanelOpen || advancedFilterCount > 0}
        class="note-list-filter-tool"
        type="button"
        aria-expanded={filterPanelOpen}
        on:click={() => (filterPanelOpen = !filterPanelOpen)}
      >
        <SlidersHorizontal size={16} strokeWidth={1.8} aria-hidden="true" />
        {t("searchFilters", $preferencesStore.language)}
        {#if advancedFilterCount > 0}
          <span>{advancedFilterCount}</span>
        {/if}
      </button>
    </div>
  </div>

  {#if filterPanelOpen}
    <div class="note-list-filter-panel">
      <label class="note-list-filter-field">
        <span>{t("titleKeyword", $preferencesStore.language)}</span>
        <input
          type="search"
          value={$notesStore.searchFilters.title}
          on:input={(event) =>
            notesStore.setSearchFilters({ title: event.currentTarget.value })}
        />
      </label>
      <section class="note-list-filter-field">
        <span>{t("tags", $preferencesStore.language)}</span>
        {#if $tagsStore.length}
          <div class="note-list-filter-tags">
            {#each $tagsStore as tag (tag.id)}
              <button
                class:active={$notesStore.searchFilters.tags.includes(tag.name)}
                type="button"
                on:click={() => void toggleSearchTag(tag.name)}
              >
                {tag.name}
              </button>
            {/each}
          </div>
        {:else}
          <p>{t("noTags", $preferencesStore.language)}</p>
        {/if}
      </section>
      <div class="note-list-filter-grid">
        <label class="note-list-filter-field">
          <span>{t("createdFrom", $preferencesStore.language)}</span>
          <input
            type="date"
            value={$notesStore.searchFilters.createdFrom}
            on:change={(event) =>
              notesStore.setSearchFilters({ createdFrom: event.currentTarget.value })}
          />
        </label>
        <label class="note-list-filter-field">
          <span>{t("createdTo", $preferencesStore.language)}</span>
          <input
            type="date"
            value={$notesStore.searchFilters.createdTo}
            on:change={(event) =>
              notesStore.setSearchFilters({ createdTo: event.currentTarget.value })}
          />
        </label>
        <label class="note-list-filter-field">
          <span>{t("updatedFrom", $preferencesStore.language)}</span>
          <input
            type="date"
            value={$notesStore.searchFilters.updatedFrom}
            on:change={(event) =>
              notesStore.setSearchFilters({ updatedFrom: event.currentTarget.value })}
          />
        </label>
        <label class="note-list-filter-field">
          <span>{t("updatedTo", $preferencesStore.language)}</span>
          <input
            type="date"
            value={$notesStore.searchFilters.updatedTo}
            on:change={(event) =>
              notesStore.setSearchFilters({ updatedTo: event.currentTarget.value })}
          />
        </label>
      </div>
      <button
        class="note-list-filter-clear"
        type="button"
        disabled={$notesStore.search === "" && advancedFilterCount === 0}
        on:click={() => void notesStore.clearSearchFilters()}
      >
        {t("clearSearchFilters", $preferencesStore.language)}
      </button>
    </div>
  {/if}

  {#if selectedCount > 0}
    <div class="note-list-selection-bar">
      <span>{selectedCount} {t("selectedNotes", $preferencesStore.language)}</span>
      <div class="note-list-selection-bar__actions">
        <button
          class="note-list-selection-tool"
          type="button"
          disabled={selectedCount === $notesStore.notes.length}
          aria-label={t("selectAll", $preferencesStore.language)}
          title={t("selectAll", $preferencesStore.language)}
          on:click={selectAllVisible}
        >
          <CheckSquare size={15} strokeWidth={2} aria-hidden="true" />
        </button>
        <button
          class="note-list-selection-tool"
          type="button"
          aria-label={t("clearAllSelection", $preferencesStore.language)}
          title={t("clearAllSelection", $preferencesStore.language)}
          on:click={clearSelection}
        >
          <Square size={15} strokeWidth={2} aria-hidden="true" />
        </button>
        <button
          class="note-list-selection-tool"
          type="button"
          aria-label={t("moveToNotebookGroup", $preferencesStore.language)}
          title={t("moveToNotebookGroup", $preferencesStore.language)}
          on:click={openMovePane}
        >
          <FolderInput size={15} strokeWidth={2} aria-hidden="true" />
        </button>
        <button
          class="danger note-list-selection-tool"
          type="button"
          aria-label={t("deleteSelectedNotes", $preferencesStore.language)}
          title={t("deleteSelectedNotes", $preferencesStore.language)}
          on:click={() => void deleteSelected()}
        >
          <Trash2 size={15} strokeWidth={2} aria-hidden="true" />
        </button>
      </div>
    </div>
  {/if}

  {#if folderViewActive && childFolders.length > 0}
    <section class="child-folder-strip" aria-label={t("childNotebookGroups", $preferencesStore.language)}>
      <h2>{t("childNotebookGroups", $preferencesStore.language)}</h2>
      <div>
        {#each childFolders as folder (folder.id)}
          <button type="button" title={folder.name} on:click={() => void selectFolder(folder.id)}>
            {#if folder.child_count > 0}
              <FolderOpen size={16} strokeWidth={1.8} aria-hidden="true" />
            {:else}
              <Folder size={16} strokeWidth={1.8} aria-hidden="true" />
            {/if}
            <span>{folder.name}</span>
            <small>
              {folder.note_count} {t("notes", $preferencesStore.language)}
              {#if folder.child_count > 0}
                · {folder.child_count} {t("folderChildGroups", $preferencesStore.language)}
              {/if}
            </small>
          </button>
        {/each}
      </div>
    </section>
  {/if}

  {#if $notesStore.filter === "starred" && starredFolders.length > 0}
    <section class="starred-folder-strip" aria-label={t("starredFolders", $preferencesStore.language)}>
      <h2>{t("starredFolders", $preferencesStore.language)}</h2>
      <div>
        {#each starredFolders as folder (folder.id)}
          <button type="button" title={folder.name} on:click={() => void selectFolder(folder.id)}>
            <span>{folder.name}</span>
            <small>{folder.note_count}</small>
          </button>
        {/each}
      </div>
    </section>
  {/if}

  {#if $notesStore.error}
    <div class="note-list-message" role="alert">{$notesStore.error}</div>
  {:else if $notesStore.loading}
    <div class="note-list-message">{t("loadingNotes", $preferencesStore.language)}</div>
  {:else if $notesStore.notes.length === 0}
    <EmptyState filter={$notesStore.filter} folderActive={Boolean($notesStore.folderID)} />
  {:else}
    <div class="note-card-list">
      {#each groupedNotes as group (group.label)}
        <section class="note-group" aria-label={group.label}>
          <h2>{group.label}</h2>
          {#each group.notes as note (note.id)}
            <NoteCard
              {note}
              active={$notesStore.selected?.id === note.id}
              selected={selectedNoteIDs.includes(note.id)}
              dragNoteIDs={selectedNoteIDs}
              onToggleSelected={toggleSelected}
              onToggleStar={toggleNoteStar}
              onDelete={deleteNoteFromList}
            />
          {/each}
        </section>
      {/each}
    </div>
  {/if}
  {/if}
</section>
