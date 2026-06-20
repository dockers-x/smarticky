<script lang="ts">
  import { Languages, Moon, Settings, Sun } from "@lucide/svelte";
  import ToolsPanel from "../settings/ToolsPanel.svelte";
  import { authStore } from "../../stores/auth";
  import { notesStore, type NoteFilter } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";

  let settingsOpen = false;

  $: filters = [
    { id: "all" as NoteFilter, label: t("allNotes", $preferencesStore.language) },
    { id: "starred" as NoteFilter, label: t("starred", $preferencesStore.language) },
    { id: "trash" as NoteFilter, label: t("trash", $preferencesStore.language) },
  ];
</script>

<aside class="sidebar" aria-label={t("noteList", $preferencesStore.language)}>
  <div class="sidebar__brand">Smarticky</div>
  <nav class="sidebar__nav">
    {#each filters as filter}
      <button
        class:active={$notesStore.filter === filter.id}
        type="button"
        aria-pressed={$notesStore.filter === filter.id}
        on:click={() => notesStore.setFilter(filter.id)}
      >
        {filter.label}
      </button>
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
    <button type="button" on:click={() => preferencesStore.toggleLanguage()}>
      <Languages size={15} strokeWidth={1.8} aria-hidden="true" />
      {$preferencesStore.language === "zh" ? "EN" : "中文"}
    </button>
  </div>
  <button
    class="sidebar__tool"
    type="button"
    aria-expanded={settingsOpen}
    on:click={() => (settingsOpen = !settingsOpen)}
  >
    <Settings size={16} strokeWidth={1.8} aria-hidden="true" />
    {t("settings", $preferencesStore.language)}
  </button>
  {#if settingsOpen}
    <ToolsPanel user={$authStore.user} onClose={() => (settingsOpen = false)} />
  {/if}
</aside>
