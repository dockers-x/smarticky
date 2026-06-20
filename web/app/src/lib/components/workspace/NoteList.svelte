<script lang="ts">
  import { Plus, Settings } from "@lucide/svelte";
  import ToolsPanel from "../settings/ToolsPanel.svelte";
  import type { Note } from "../../api/types";
  import { authStore } from "../../stores/auth";
  import { confirmDialog, notify } from "../../stores/dialogs";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";
  import EmptyState from "./EmptyState.svelte";
  import NoteCard from "./NoteCard.svelte";

  interface NoteGroup {
    label: string;
    notes: Note[];
  }

  let settingsOpen = false;

  $: filters = [
    { id: "all" as const, label: t("allNotes", $preferencesStore.language) },
    { id: "starred" as const, label: t("starred", $preferencesStore.language) },
    { id: "trash" as const, label: t("trash", $preferencesStore.language) },
  ];

  function groupLabel(date: Date, language: "zh" | "en"): string {
    const today = new Date();
    const yesterday = new Date();
    yesterday.setDate(today.getDate() - 1);

    const sameDay = (left: Date, right: Date) =>
      left.getFullYear() === right.getFullYear() &&
      left.getMonth() === right.getMonth() &&
      left.getDate() === right.getDate();

    if (sameDay(date, today)) return t("today", language);
    if (sameDay(date, yesterday)) return t("yesterday", language);

    return date.toLocaleDateString(
      language === "zh" ? "zh-CN" : "en-US",
      {
        month: "long",
        day: "numeric",
        year: date.getFullYear() === today.getFullYear() ? undefined : "numeric",
      },
    );
  }

  $: groupedNotes = $notesStore.notes.reduce<NoteGroup[]>((groups, note) => {
    const label = groupLabel(new Date(note.updated_at), $preferencesStore.language);
    const group = groups.find((item) => item.label === label);
    if (group) {
      group.notes.push(note);
    } else {
      groups.push({ label, notes: [note] });
    }
    return groups;
  }, []);

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
        class="note-list-mobile-tool"
        type="button"
        aria-expanded={settingsOpen}
        on:click={() => (settingsOpen = !settingsOpen)}
      >
        <Settings size={16} strokeWidth={1.8} aria-hidden="true" />
        {t("settings", $preferencesStore.language)}
      </button>
    </div>
  </div>

  <div class="note-list-mobile-filters" aria-label={t("noteList", $preferencesStore.language)}>
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
  </div>

  {#if settingsOpen}
    <ToolsPanel user={$authStore.user} onClose={() => (settingsOpen = false)} />
  {/if}

  {#if $notesStore.error}
    <div class="note-list-message" role="alert">{$notesStore.error}</div>
  {:else if $notesStore.loading}
    <div class="note-list-message">{t("loadingNotes", $preferencesStore.language)}</div>
  {:else if $notesStore.notes.length === 0}
    <EmptyState filter={$notesStore.filter} />
  {:else}
    <div class="note-card-list">
      {#each groupedNotes as group (group.label)}
        <section class="note-group" aria-label={group.label}>
          <h2>{group.label}</h2>
          {#each group.notes as note (note.id)}
            <NoteCard {note} active={$notesStore.selected?.id === note.id} />
          {/each}
        </section>
      {/each}
    </div>
  {/if}

  {#if $notesStore.filter !== "trash"}
    <button
      class="new-note-fab"
      type="button"
      aria-label={t("newNote", $preferencesStore.language)}
      on:click={() => notesStore.create()}
    >
      <Plus size={25} strokeWidth={2.1} aria-hidden="true" />
    </button>
  {/if}
</section>
