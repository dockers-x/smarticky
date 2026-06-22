<script lang="ts">
  import {
    ChevronDown,
    ChevronRight,
    Folder,
    FolderOpen,
    MoreHorizontal,
    Pencil,
    Plus,
    Star,
    Trash2,
  } from "@lucide/svelte";
  import type { Folder as FolderType } from "../../api/types";
  import { confirmDialog, inputDialog, notify } from "../../stores/dialogs";
  import { buildFolderTree, foldersStore, type FolderTreeItem } from "../../stores/folders";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";

  export let selectedNoteIDs: string[] = [];
  export let onSelectionMoved: () => void = () => {};

  interface FolderRow {
    folder: FolderType;
    depth: number;
    hasChildren: boolean;
  }

  let dragOverFolderID: string | null = null;
  let draggedFolderID: string | null = null;
  let expandTimer: ReturnType<typeof window.setTimeout> | null = null;
  let activeFolderMenuID: string | null = null;

  $: selectedCount = selectedNoteIDs.length;
  $: folderTree = buildFolderTree($foldersStore.folders);
  $: folderRows = visibleFolderRows(folderTree, $foldersStore.expandedFolderIDs);

  function visibleFolderRows(
    items: FolderTreeItem[],
    expandedIDs: string[],
  ): FolderRow[] {
    const rows: FolderRow[] = [];
    const visit = (nodes: FolderTreeItem[]) => {
      for (const item of nodes) {
        rows.push({
          folder: item.folder,
          depth: item.depth,
          hasChildren: item.children.length > 0,
        });
        if (expandedIDs.includes(item.folder.id)) visit(item.children);
      }
    };
    visit(items);
    return rows;
  }

  async function createNotebookGroup(parentID: string | null = null): Promise<void> {
    if (parentID) {
      const parent = $foldersStore.folders.find((folder) => folder.id === parentID);
      if (parent && parent.depth >= $foldersStore.settings.max_depth) {
        notify(t("folderDepthLimit", $preferencesStore.language), "info");
        return;
      }
    }

    const name = await inputDialog({
      title: t("newNotebookGroup", $preferencesStore.language),
      label: t("folderName", $preferencesStore.language),
      confirmLabel: t("add", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
      requiredMessage: t("folderNameRequired", $preferencesStore.language),
    });
    if (!name) return;

    try {
      const folder = await foldersStore.create({ name, parent_id: parentID });
      foldersStore.select(folder.id);
      activeFolderMenuID = null;
      notify(t("folderCreated", $preferencesStore.language), "success");
    } catch {
      notify(t("folderCreateFailed", $preferencesStore.language), "error");
    }
  }

  async function renameFolder(folder: FolderType): Promise<void> {
    const name = await inputDialog({
      title: t("renameNotebookGroup", $preferencesStore.language),
      label: t("folderName", $preferencesStore.language),
      initialValue: folder.name,
      confirmLabel: t("done", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
      requiredMessage: t("folderNameRequired", $preferencesStore.language),
    });
    if (!name || name === folder.name) return;

    try {
      await foldersStore.updateFolder(folder.id, { name });
      activeFolderMenuID = null;
    } catch {
      notify(t("folderRenameFailed", $preferencesStore.language), "error");
    }
  }

  function folderContains(folderID: string, possibleDescendantID: string): boolean {
    const byID = new Map($foldersStore.folders.map((folder) => [folder.id, folder]));
    let current = byID.get(possibleDescendantID);
    while (current?.parent_id) {
      if (current.parent_id === folderID) return true;
      current = byID.get(current.parent_id);
    }
    return false;
  }

  function droppedFolderID(event: DragEvent): string | null {
    const folderID = event.dataTransfer?.getData("application/x-smarticky-folder-id");
    return folderID || null;
  }

  function canDropFolder(folderID: string, targetParentID: string | null): boolean {
    if (folderID === targetParentID) return false;
    if (targetParentID && folderContains(folderID, targetParentID)) return false;

    const folder = $foldersStore.folders.find((item) => item.id === folderID);
    if (!folder) return false;
    return (folder.parent_id ?? null) !== targetParentID;
  }

  function beginFolderDrag(event: DragEvent, folder: FolderType): void {
    draggedFolderID = folder.id;
    event.dataTransfer?.setData("application/x-smarticky-folder-id", folder.id);
    if (event.dataTransfer) event.dataTransfer.effectAllowed = "move";
  }

  function clearExpandTimer(): void {
    if (!expandTimer) return;
    window.clearTimeout(expandTimer);
    expandTimer = null;
  }

  function scheduleExpandOnDrag(folderID: string): void {
    const folder = $foldersStore.folders.find((item) => item.id === folderID);
    if (!folder || folder.child_count === 0) return;
    if ($foldersStore.expandedFolderIDs.includes(folderID)) return;

    clearExpandTimer();
    expandTimer = window.setTimeout(() => {
      foldersStore.toggleExpanded(folderID);
      expandTimer = null;
    }, 520);
  }

  function clearDragState(): void {
    dragOverFolderID = null;
    draggedFolderID = null;
    clearExpandTimer();
  }

  function clearDragTarget(): void {
    dragOverFolderID = null;
    clearExpandTimer();
  }

  function handleFolderDragOver(event: DragEvent, targetParentID: string | null): void {
    const folderID = draggedFolderID;
    if (!folderID || !canDropFolder(folderID, targetParentID)) return;

    event.preventDefault();
    dragOverFolderID = targetParentID ?? "root";
    if (event.dataTransfer) event.dataTransfer.dropEffect = "move";
    if (targetParentID) scheduleExpandOnDrag(targetParentID);
  }

  function handleFolderRowDragOver(event: DragEvent, folderID: string): void {
    if (draggedFolderID) {
      handleFolderDragOver(event, folderID);
      return;
    }

    if (!event.dataTransfer?.types.includes("application/x-smarticky-note-ids")) return;

    event.preventDefault();
    dragOverFolderID = folderID;
    if (event.dataTransfer) event.dataTransfer.dropEffect = "move";
    scheduleExpandOnDrag(folderID);
  }

  async function moveDroppedFolder(event: DragEvent, targetParentID: string | null): Promise<boolean> {
    const folderID = droppedFolderID(event);
    if (!folderID) return false;

    event.preventDefault();
    clearDragState();

    if (!canDropFolder(folderID, targetParentID)) return true;

    const folder = $foldersStore.folders.find((item) => item.id === folderID);
    if (!folder) return true;

    if (folder.note_count > 0 || folder.child_count > 0) {
      const confirmed = await confirmDialog({
        title: t("moveNotebookGroup", $preferencesStore.language),
        message: t("folderMoveNonEmptyMessage", $preferencesStore.language),
        confirmLabel: t("moveNotebookGroup", $preferencesStore.language),
        cancelLabel: t("cancel", $preferencesStore.language),
      });
      if (!confirmed) return true;
    }

    try {
      await foldersStore.updateFolder(folderID, { parent_id: targetParentID });
      await foldersStore.load();
      notify(t("folderMoved", $preferencesStore.language), "success");
      activeFolderMenuID = null;
    } catch {
      notify(t("folderMoveFailed", $preferencesStore.language), "error");
    }
    return true;
  }

  function droppedNoteIDs(event: DragEvent): string[] {
    const raw = event.dataTransfer?.getData("application/x-smarticky-note-ids");
    if (!raw) return [];
    try {
      const parsed = JSON.parse(raw);
      return Array.isArray(parsed)
        ? parsed.filter((item): item is string => typeof item === "string")
        : [];
    } catch {
      return [];
    }
  }

  async function moveNotesToFolder(noteIDs: string[], folderID: string | null): Promise<void> {
    if (noteIDs.length === 0) return;

    try {
      await notesStore.moveToFolder(noteIDs, folderID);
      onSelectionMoved();
      foldersStore.select(folderID);
      await Promise.all([foldersStore.load(), notesStore.setFolder(folderID ?? "unfiled")]);
      notify(t("movedNotes", $preferencesStore.language), "success");
    } catch {
      notify(t("moveNotesFailed", $preferencesStore.language), "error");
    }
  }

  async function moveDroppedNotes(event: DragEvent, folderID: string | null): Promise<void> {
    event.preventDefault();
    dragOverFolderID = null;
    clearExpandTimer();
    await moveNotesToFolder(droppedNoteIDs(event), folderID);
  }

  async function handleDropOnFolder(event: DragEvent, folderID: string | null): Promise<void> {
    if (await moveDroppedFolder(event, folderID)) return;
    await moveDroppedNotes(event, folderID);
  }

  async function activateFolder(folderID: string | null): Promise<void> {
    activeFolderMenuID = null;
    if (selectedCount > 0) {
      await moveNotesToFolder(selectedNoteIDs, folderID);
      return;
    }

    foldersStore.select(folderID);
    await notesStore.setFolder(folderID ?? "unfiled");
  }

  async function toggleFolderStar(folder: FolderType): Promise<void> {
    try {
      await foldersStore.updateFolder(folder.id, {
        is_starred: !folder.is_starred,
      });
      activeFolderMenuID = null;
    } catch {
      notify(t("folderStarFailed", $preferencesStore.language), "error");
    }
  }

  async function removeFolder(folder: FolderType): Promise<void> {
    const confirmed = await confirmDialog({
      title: t("delete", $preferencesStore.language),
      message: t("folderDeleteMessage", $preferencesStore.language),
      confirmLabel: t("delete", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
    });
    if (!confirmed) return;

    try {
      const wasActive = $foldersStore.activeFolderID === folder.id;
      await foldersStore.delete(folder.id);
      if (wasActive) {
        await notesStore.setFilter("all");
      }
      notify(t("folderDeleted", $preferencesStore.language), "success");
      activeFolderMenuID = null;
    } catch {
      notify(t("folderDeleteFailed", $preferencesStore.language), "error");
    }
  }
</script>

<section class="folder-browser-pane" aria-label={t("notebookGroups", $preferencesStore.language)}>
  <div
    class:drop-target={dragOverFolderID === "root"}
    class="folder-browser-header"
    role="group"
    aria-label={t("notebookGroups", $preferencesStore.language)}
    on:dragover={(event) => handleFolderDragOver(event, null)}
    on:dragleave={clearDragTarget}
    on:drop={(event) => void moveDroppedFolder(event, null)}
  >
    <div>
      <h1>{t("notebookGroups", $preferencesStore.language)}</h1>
      <span>
        {selectedCount > 0
          ? `${selectedCount} ${t("selectedNotes", $preferencesStore.language)}`
          : `${$foldersStore.folders.length} ${t("notebookGroups", $preferencesStore.language)}`}
      </span>
    </div>
    <button
      class="folder-browser-header__new"
      type="button"
      aria-label={t("newNotebookGroup", $preferencesStore.language)}
      on:click={() => void createNotebookGroup()}
    >
      <Plus size={18} strokeWidth={2} aria-hidden="true" />
    </button>
  </div>

  {#if selectedCount > 0}
    <p class="folder-browser-move-hint">
      {t("moveToNotebookGroup", $preferencesStore.language)}
    </p>
  {/if}

  {#if $foldersStore.loading}
    <div class="folder-browser-message">{t("loading", $preferencesStore.language)}</div>
  {:else if $foldersStore.error}
    <div class="folder-browser-message error">{$foldersStore.error}</div>
  {:else}
    <div class="folder-browser-tree" role="list">
      {#each folderRows as row (row.folder.id)}
        <div
          class:active={$foldersStore.activeFolderID === row.folder.id}
          class:drop-target={dragOverFolderID === row.folder.id}
          class:folder-browser-row--deep={row.depth >= 3}
          class:move-target={selectedCount > 0}
          class="folder-browser-row"
          role="listitem"
          draggable={true}
          aria-label={row.folder.name}
          style={`--folder-depth: ${row.depth - 1}`}
          on:dragstart={(event) => beginFolderDrag(event, row.folder)}
          on:dragend={clearDragState}
          on:dragover={(event) => handleFolderRowDragOver(event, row.folder.id)}
          on:dragleave={clearDragTarget}
          on:drop={(event) => void handleDropOnFolder(event, row.folder.id)}
        >
          <button
            class="folder-browser-row__chevron"
            type="button"
            aria-label={row.hasChildren ? row.folder.name : undefined}
            aria-expanded={row.hasChildren
              ? $foldersStore.expandedFolderIDs.includes(row.folder.id)
              : undefined}
            disabled={!row.hasChildren}
            on:click|stopPropagation={() => foldersStore.toggleExpanded(row.folder.id)}
          >
            {#if row.hasChildren && $foldersStore.expandedFolderIDs.includes(row.folder.id)}
              <ChevronDown size={15} strokeWidth={2} aria-hidden="true" />
            {:else}
              <ChevronRight size={15} strokeWidth={2} aria-hidden="true" />
            {/if}
          </button>
          <button
            class="folder-browser-row__select"
            type="button"
            title={row.folder.name}
            aria-pressed={$foldersStore.activeFolderID === row.folder.id}
            on:click={() => void activateFolder(row.folder.id)}
          >
            {#if row.hasChildren && $foldersStore.expandedFolderIDs.includes(row.folder.id)}
              <FolderOpen size={17} strokeWidth={1.8} aria-hidden="true" />
            {:else}
              <Folder size={17} strokeWidth={1.8} aria-hidden="true" />
            {/if}
            <span class="folder-browser-row__name">{row.folder.name}</span>
            <span class="folder-browser-row__count">{row.folder.note_count}</span>
          </button>
          <div class="folder-browser-row__actions">
            <button
              class="folder-browser-row__more"
              type="button"
              aria-label={t("moreActions", $preferencesStore.language)}
              aria-expanded={activeFolderMenuID === row.folder.id}
              title={t("moreActions", $preferencesStore.language)}
              on:click|stopPropagation={() =>
                (activeFolderMenuID =
                  activeFolderMenuID === row.folder.id ? null : row.folder.id)}
            >
              <MoreHorizontal size={16} strokeWidth={2} aria-hidden="true" />
            </button>
            {#if activeFolderMenuID === row.folder.id}
              <div class="folder-browser-menu">
                <button
                  type="button"
                  disabled={row.folder.depth >= $foldersStore.settings.max_depth}
                  on:click|stopPropagation={() => void createNotebookGroup(row.folder.id)}
                >
                  <Plus size={14} strokeWidth={2} aria-hidden="true" />
                  {t("newNotebookGroup", $preferencesStore.language)}
                </button>
                <button
                  type="button"
                  aria-pressed={row.folder.is_starred}
                  on:click|stopPropagation={() => void toggleFolderStar(row.folder)}
                >
                  <Star
                    size={14}
                    strokeWidth={2}
                    fill={row.folder.is_starred ? "currentColor" : "none"}
                    aria-hidden="true"
                  />
                  {t("star", $preferencesStore.language)}
                </button>
                <button
                  type="button"
                  on:click|stopPropagation={() => void renameFolder(row.folder)}
                >
                  <Pencil size={14} strokeWidth={2} aria-hidden="true" />
                  {t("renameNotebookGroup", $preferencesStore.language)}
                </button>
                <button
                  class="danger"
                  type="button"
                  on:click|stopPropagation={() => void removeFolder(row.folder)}
                >
                  <Trash2 size={14} strokeWidth={2} aria-hidden="true" />
                  {t("delete", $preferencesStore.language)}
                </button>
              </div>
            {/if}
          </div>
        </div>
      {/each}

      <button
        class:active={$notesStore.folderID === "unfiled"}
        class:drop-target={dragOverFolderID === "unfiled"}
        class:move-target={selectedCount > 0}
        class="folder-browser-row folder-browser-row--unfiled"
        type="button"
        on:dragover|preventDefault={() => (dragOverFolderID = "unfiled")}
        on:dragleave={clearDragTarget}
        on:drop={(event) => void moveDroppedNotes(event, null)}
        on:click={() => void activateFolder(null)}
      >
        <Folder size={17} strokeWidth={1.8} aria-hidden="true" />
        <span>{t("unfiledNotes", $preferencesStore.language)}</span>
      </button>
    </div>
  {/if}
</section>
