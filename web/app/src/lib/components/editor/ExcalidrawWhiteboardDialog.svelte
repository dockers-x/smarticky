<script lang="ts">
  import { Trash2, X } from "@lucide/svelte";
  import { onDestroy, onMount, tick } from "svelte";
  import { get } from "svelte/store";
  import type { Whiteboard } from "../../api/types";
  import {
    getExcalidrawLibrary,
    updateExcalidrawLibrary,
  } from "../../api/excalidrawLibrary";
  import {
    deleteWhiteboard,
    getWhiteboard,
    updateWhiteboard,
  } from "../../api/whiteboards";
  import { excalidrawLibraryReturnURL } from "../../excalidraw/library";
  import { removeWhiteboardReferenceFences } from "../../markdown/whiteboards";
  import {
    fontFamilyValue,
    fontsStore,
    DEFAULT_FONT,
    systemFontOptions,
  } from "../../stores/fonts";
  import { confirmDialog, notify } from "../../stores/dialogs";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";
  import { whiteboardStore } from "../../stores/whiteboard";
  import {
    setStoredSceneFontFamily,
    storedSceneFontFamily,
  } from "../../excalidraw/scene";
  import type {
    ExcalidrawMountHandle,
    ExcalidrawMountOptions,
  } from "../../excalidraw/ExcalidrawIsland";

  export let whiteboardID = "";
  export let onClose: () => void = () => {};

  type SaveStatus = "idle" | "saving" | "saved" | "error";

  let host: HTMLDivElement | null = null;
  let mountHandle: ExcalidrawMountHandle | null = null;
  let whiteboard: Whiteboard | null = null;
  let loading = true;
  let error = "";
  let draftTitle = "";
  let currentSceneJSON = "{}";
  let lastSavedSceneJSON = "{}";
  let currentLibraryJSON = "[]";
  let lastSavedLibraryJSON = "[]";
  let selectedFont = DEFAULT_FONT;
  let saveStatus: SaveStatus = "idle";
  let deleting = false;
  let saveTimer: ReturnType<typeof setTimeout> | null = null;
  let librarySaveTimer: ReturnType<typeof setTimeout> | null = null;
  let saveSequence = 0;
  let librarySaveSequence = 0;
  let lastLibraryRefreshToken = 0;
  let islandModule:
    | typeof import("../../excalidraw/ExcalidrawIsland")
    | null = null;

  $: statusText = {
    idle: "",
    saving: t("saving", $preferencesStore.language),
    saved: t("saved", $preferencesStore.language),
    error: t("saveError", $preferencesStore.language),
  } satisfies Record<SaveStatus, string>;

  $: uploadedFontOptions = $fontsStore.fonts.map((font) => ({
    name: font.name,
    displayName: font.display_name,
  }));

  $: whiteboardFontStyle = `font-family: ${fontFamilyValue(selectedFont)};`;

  function clearSaveTimer(): void {
    if (saveTimer) clearTimeout(saveTimer);
    saveTimer = null;
  }

  function clearLibrarySaveTimer(): void {
    if (librarySaveTimer) clearTimeout(librarySaveTimer);
    librarySaveTimer = null;
  }

  function mountOptions(): ExcalidrawMountOptions {
    return {
      sceneJSON: currentSceneJSON,
      title: draftTitle.trim() || t("whiteboard", $preferencesStore.language),
      theme: $preferencesStore.theme,
      fontFamily: selectedFont,
      libraryJSON: currentLibraryJSON,
      libraryReturnUrl: excalidrawLibraryReturnURL(whiteboardID),
      onChange: handleSceneChange,
      onLibraryChange: handleLibraryChange,
    };
  }

  async function ensureMounted(): Promise<void> {
    if (!host || !whiteboard || mountHandle) return;
    islandModule ??= await import("../../excalidraw/ExcalidrawIsland");
    if (!host || !whiteboard || mountHandle) return;
    mountHandle = islandModule.mountExcalidrawWhiteboard(host, mountOptions());
  }

  async function loadWhiteboard(): Promise<void> {
    loading = true;
    error = "";
    try {
      await fontsStore.load();
      const [loaded, library] = await Promise.all([
        getWhiteboard(whiteboardID),
        getExcalidrawLibrary(),
      ]);
      whiteboard = loaded;
      draftTitle = loaded.title || t("whiteboard", $preferencesStore.language);
      currentSceneJSON = loaded.scene_json || "{}";
      lastSavedSceneJSON = currentSceneJSON;
      currentLibraryJSON = library.library_json || "[]";
      lastSavedLibraryJSON = currentLibraryJSON;
      selectedFont = storedSceneFontFamily(
        currentSceneJSON,
        get(fontsStore).selected || DEFAULT_FONT,
      );
      saveStatus = "saved";
      await tick();
      await ensureMounted();
    } catch (loadError) {
      error =
        loadError instanceof Error
          ? loadError.message
          : t("whiteboardLoadFailed", $preferencesStore.language);
    } finally {
      loading = false;
    }
  }

  function queueSave(): void {
    if (!whiteboard) return;
    saveStatus = "saving";
    clearSaveTimer();
    saveTimer = setTimeout(() => {
      void saveNow();
    }, 700);
  }

  function queueLibrarySave(): void {
    if (!whiteboard) return;
    clearLibrarySaveTimer();
    librarySaveTimer = setTimeout(() => {
      void saveLibraryNow();
    }, 700);
  }

  function handleSceneChange(sceneJSON: string): void {
    if (sceneJSON === currentSceneJSON) return;
    currentSceneJSON = sceneJSON;
    queueSave();
  }

  function handleLibraryChange(libraryJSON: string): void {
    if (libraryJSON === currentLibraryJSON) return;
    currentLibraryJSON = libraryJSON;
    queueLibrarySave();
  }

  function updateTitle(value: string): void {
    draftTitle = value;
    mountHandle?.update(mountOptions());
    queueSave();
  }

  function updateFont(value: string): void {
    selectedFont = value || DEFAULT_FONT;
    currentSceneJSON = setStoredSceneFontFamily(currentSceneJSON, selectedFont);
    mountHandle?.update(mountOptions());
    queueSave();
  }

  async function saveNow(): Promise<void> {
    clearSaveTimer();
    if (!whiteboard) return;

    const title = draftTitle.trim() || t("whiteboard", $preferencesStore.language);
    const payload: { title?: string; scene_json?: string } = {};
    if (title !== whiteboard.title) payload.title = title;
    if (currentSceneJSON !== lastSavedSceneJSON) {
      payload.scene_json = currentSceneJSON;
    }

    if (!payload.title && !payload.scene_json) {
      saveStatus = "saved";
      return;
    }

    const submittedTitle = title;
    const submittedSceneJSON = currentSceneJSON;
    const sequence = ++saveSequence;
    saveStatus = "saving";
    try {
      const updated = await updateWhiteboard(whiteboard.id, payload);
      if (sequence === saveSequence) {
        whiteboard = updated;
        if (submittedSceneJSON === currentSceneJSON) {
          lastSavedSceneJSON = updated.scene_json;
          currentSceneJSON = updated.scene_json;
        } else {
          lastSavedSceneJSON = submittedSceneJSON;
        }
        if (draftTitle.trim() === submittedTitle) {
          draftTitle = updated.title;
        }
        saveStatus = saveTimer ? "saving" : "saved";
      }
    } catch {
      if (sequence === saveSequence) {
        saveStatus = "error";
        notify(t("whiteboardSaveFailed", $preferencesStore.language), "error");
      }
    }
  }

  async function saveLibraryNow(): Promise<void> {
    clearLibrarySaveTimer();
    if (currentLibraryJSON === lastSavedLibraryJSON) return;

    const submittedLibraryJSON = currentLibraryJSON;
    const sequence = ++librarySaveSequence;
    try {
      const updated = await updateExcalidrawLibrary({
        library_json: submittedLibraryJSON,
      });
      if (sequence === librarySaveSequence) {
        if (submittedLibraryJSON === currentLibraryJSON) {
          lastSavedLibraryJSON = updated.library_json;
          currentLibraryJSON = updated.library_json;
        } else {
          lastSavedLibraryJSON = submittedLibraryJSON;
        }
      }
    } catch {
      if (sequence === librarySaveSequence) {
        notify(t("whiteboardSaveFailed", $preferencesStore.language), "error");
      }
    }
  }

  async function refreshLibraryFromServer(): Promise<void> {
    try {
      const library = await getExcalidrawLibrary();
      currentLibraryJSON = library.library_json || "[]";
      lastSavedLibraryJSON = currentLibraryJSON;
      mountHandle?.updateLibrary(currentLibraryJSON);
    } catch {
      notify(t("excalidrawLibraryImportFailed", $preferencesStore.language), "error");
    }
  }

  async function closeDialog(): Promise<void> {
    try {
      await Promise.all([saveNow(), saveLibraryNow()]);
    } finally {
      onClose();
    }
  }

  async function removeReferenceFromOwningNote(target: Whiteboard): Promise<number> {
    if (!target.note_id) return 0;

    const draftRemoval = await whiteboardStore.removeReference(
      target.id,
      target.note_id,
    );
    if (draftRemoval.handled) return draftRemoval.removedCount;

    const ownerNote = await notesStore.getByID(target.note_id);
    const removal = removeWhiteboardReferenceFences(ownerNote.content, target.id);
    if (removal.removedCount === 0) return 0;

    await notesStore.updateNote(target.note_id, { content: removal.markdown });
    return removal.removedCount;
  }

  async function deleteCurrentWhiteboard(): Promise<void> {
    if (!whiteboard || deleting) return;

    const confirmed = await confirmDialog({
      title: t("whiteboardDeleteConfirm", $preferencesStore.language),
      message: t("whiteboardDeleteMessage", $preferencesStore.language),
      confirmLabel: t("delete", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
    });
    if (!confirmed) return;

    deleting = true;
    try {
      const removedCount = await removeReferenceFromOwningNote(whiteboard);
      await saveLibraryNow();
      await deleteWhiteboard(whiteboard.id);
      notify(
        removedCount > 0
          ? t("whiteboardDeletedWithReference", $preferencesStore.language)
          : t("whiteboardDeleted", $preferencesStore.language),
        "success",
      );
      onClose();
    } catch {
      notify(t("whiteboardDeleteFailed", $preferencesStore.language), "error");
    } finally {
      deleting = false;
    }
  }

  onMount(() => {
    void loadWhiteboard();
  });

  $: if (host && whiteboard && !mountHandle) {
    void ensureMounted();
  }

  $: if (
    $whiteboardStore.libraryRefreshToken !== lastLibraryRefreshToken &&
    $whiteboardStore.libraryRefreshID === whiteboardID
  ) {
    lastLibraryRefreshToken = $whiteboardStore.libraryRefreshToken;
    void refreshLibraryFromServer();
  }

  onDestroy(() => {
    clearSaveTimer();
    clearLibrarySaveTimer();
    mountHandle?.destroy();
    mountHandle = null;
  });
</script>

<div
  class="whiteboard-dialog-backdrop"
  role="presentation"
  on:click={(event) => {
    if (!deleting && event.currentTarget === event.target) void closeDialog();
  }}
>
  <div
    class="whiteboard-dialog"
    role="dialog"
    aria-modal="true"
    aria-label={t("whiteboard", $preferencesStore.language)}
  >
    <header class="whiteboard-dialog__header">
      <div class="whiteboard-dialog__title">
        <input
          value={draftTitle}
          aria-label={t("whiteboardTitle", $preferencesStore.language)}
          placeholder={t("whiteboard", $preferencesStore.language)}
          on:input={(event) => updateTitle(event.currentTarget.value)}
        />
        <span class:visible={saveStatus !== "idle"}>{statusText[saveStatus]}</span>
      </div>
      <label class="whiteboard-dialog__font">
        <span>{t("selectedFont", $preferencesStore.language)}</span>
        <select
          value={selectedFont}
          on:change={(event) => updateFont(event.currentTarget.value)}
        >
          <optgroup label={t("systemFonts", $preferencesStore.language)}>
            {#each systemFontOptions as option}
              <option value={option.name}>
                {option.name === DEFAULT_FONT
                  ? t("systemDefault", $preferencesStore.language)
                  : option.displayName}
              </option>
            {/each}
          </optgroup>
          {#if uploadedFontOptions.length}
            <optgroup label={t("uploadedFonts", $preferencesStore.language)}>
              {#each uploadedFontOptions as font (font.name)}
                <option value={font.name}>{font.displayName}</option>
              {/each}
            </optgroup>
          {/if}
        </select>
      </label>
      <div class="whiteboard-dialog__actions">
        <button
          class="whiteboard-dialog__icon-button danger"
          type="button"
          aria-label={t("whiteboardDeleteConfirm", $preferencesStore.language)}
          disabled={!whiteboard || deleting}
          on:click={() => void deleteCurrentWhiteboard()}
        >
          <Trash2 aria-hidden="true" size={18} strokeWidth={2} />
        </button>
        <button
          class="whiteboard-dialog__icon-button"
          type="button"
          aria-label={t("closeWhiteboard", $preferencesStore.language)}
          disabled={deleting}
          on:click={() => void closeDialog()}
        >
          <X aria-hidden="true" size={19} strokeWidth={2} />
        </button>
      </div>
    </header>

    <div class="whiteboard-dialog__body" style={whiteboardFontStyle}>
      {#if loading}
        <div class="whiteboard-dialog__state" role="status">
          {t("loading", $preferencesStore.language)}
        </div>
      {:else if error}
        <div class="whiteboard-dialog__state error" role="alert">{error}</div>
      {:else}
        <div class="whiteboard-dialog__canvas" bind:this={host}></div>
      {/if}
    </div>
  </div>
</div>
