<script lang="ts">
  import { onMount } from "svelte";
  import DialogHost from "./lib/components/common/DialogHost.svelte";
  import Workspace from "./lib/components/workspace/Workspace.svelte";
  import {
    importExcalidrawLibraryFromCallback,
    whiteboardIDFromLibraryCallback,
  } from "./lib/excalidraw/library";
  import { authStore } from "./lib/stores/auth";
  import { notify } from "./lib/stores/dialogs";
  import { preferencesStore, t } from "./lib/stores/preferences";
  import { whiteboardStore } from "./lib/stores/whiteboard";

  let libraryCallbackRunning = false;

  async function handleLibraryCallback(): Promise<void> {
    if (!$authStore.user || libraryCallbackRunning) return;

    libraryCallbackRunning = true;
    const callbackWhiteboardID = whiteboardIDFromLibraryCallback();
    try {
      const result = await importExcalidrawLibraryFromCallback();
      if (result.whiteboardID) {
        whiteboardStore.open(result.whiteboardID);
        whiteboardStore.refreshLibrary(result.whiteboardID);
      }
      if (result.imported) {
        notify(t("excalidrawLibraryImported", $preferencesStore.language), "success");
      }
    } catch {
      if (callbackWhiteboardID) {
        whiteboardStore.open(callbackWhiteboardID);
      }
      notify(t("excalidrawLibraryImportFailed", $preferencesStore.language), "error");
    } finally {
      libraryCallbackRunning = false;
    }
  }

  onMount(() => {
    preferencesStore.hydrate();
    authStore.hydrate();

    const viewport = window.visualViewport;
    const updateViewportHeight = () => {
      if (viewport && viewport.scale !== 1) return;
      document.documentElement.style.setProperty(
        "--app-height",
        `${Math.round(viewport?.height ?? window.innerHeight)}px`,
      );
    };
    const handleKeyboard = () => {
      document.documentElement.dataset.inputMethod = "keyboard";
    };
    const handlePointer = () => {
      document.documentElement.dataset.inputMethod = "pointer";
    };
    updateViewportHeight();
    viewport?.addEventListener("resize", updateViewportHeight);
    window.addEventListener("resize", updateViewportHeight);
    window.addEventListener("keydown", handleKeyboard, true);
    window.addEventListener("pointerdown", handlePointer, true);

    const onHashChange = () => {
      void handleLibraryCallback();
    };
    window.addEventListener("hashchange", onHashChange);
    return () => {
      window.removeEventListener("hashchange", onHashChange);
      viewport?.removeEventListener("resize", updateViewportHeight);
      window.removeEventListener("resize", updateViewportHeight);
      window.removeEventListener("keydown", handleKeyboard, true);
      window.removeEventListener("pointerdown", handlePointer, true);
      document.documentElement.style.removeProperty("--app-height");
    };
  });

  $: if ($authStore.user) {
    void handleLibraryCallback();
  }
</script>

{#if $authStore.loading}
  <main class="app-shell" aria-label="Smarticky workspace">
    <section class="boot-panel">
      <p class="boot-kicker">SMARTICKY</p>
      <h1>Smarticky</h1>
      <p>{t("preparing", $preferencesStore.language)}</p>
    </section>
  </main>
{:else if $authStore.user}
  <Workspace />
{/if}

<DialogHost />
