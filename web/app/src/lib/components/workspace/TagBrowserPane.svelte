<script lang="ts">
  import { onMount } from "svelte";
  import { FileText, Search, Tag } from "@lucide/svelte";
  import type { Tag as TagType } from "../../api/types";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";
  import { tagsStore } from "../../stores/tags";

  interface TagGroup {
    label: string;
    tags: TagType[];
  }

  let query = "";

  onMount(() => {
    void tagsStore.load();
  });

  $: normalizedQuery = query.trim().toLowerCase();
  $: visibleTags = $tagsStore.filter((tag) =>
    tag.name.toLowerCase().includes(normalizedQuery),
  );
  $: groupedTags = groupTags(visibleTags);
  $: activeTagNames = $notesStore.searchFilters.tags;

  function groupLabel(name: string): string {
    const first = name.trim().charAt(0).toUpperCase();
    if (!first) return "#";
    return /^[A-Z0-9]$/.test(first) ? first : "#";
  }

  function groupTags(tags: TagType[]): TagGroup[] {
    const groups = new Map<string, TagType[]>();
    for (const tag of tags) {
      const label = groupLabel(tag.name);
      groups.set(label, [...(groups.get(label) ?? []), tag]);
    }

    return [...groups.entries()]
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([label, items]) => ({
        label,
        tags: items.sort((left, right) => left.name.localeCompare(right.name)),
      }));
  }

  async function selectTag(name: string): Promise<void> {
    await notesStore.setTagFilter([name]);
  }

  async function clearTags(): Promise<void> {
    await notesStore.setTagFilter([]);
  }
</script>

<section class="tag-browser-pane" aria-label={t("tags", $preferencesStore.language)}>
  <div class="tag-browser-header">
    <div>
      <h1>{t("tags", $preferencesStore.language)}</h1>
      <span>
        {activeTagNames.length > 0
          ? activeTagNames.join(", ")
          : `${$tagsStore.length} ${t("tags", $preferencesStore.language)}`}
      </span>
    </div>
  </div>

  <label class="browser-search tag-browser-search">
    <Search size={15} strokeWidth={1.8} aria-hidden="true" />
    <input
      type="search"
      bind:value={query}
      placeholder={t("searchTags", $preferencesStore.language)}
      aria-label={t("searchTags", $preferencesStore.language)}
    />
  </label>

  <div class="tag-browser-list" role="list">
    {#if activeTagNames.length > 0}
      <button
        class="tag-browser-row tag-browser-row--quick"
        type="button"
        on:click={() => void clearTags()}
      >
        <FileText size={16} strokeWidth={1.8} aria-hidden="true" />
        <span>{t("clearTagFilter", $preferencesStore.language)}</span>
      </button>
    {/if}

    {#if $tagsStore.length === 0}
      <div class="tag-browser-empty">
        <p>{t("noTags", $preferencesStore.language)}</p>
      </div>
    {:else if groupedTags.length === 0}
      <div class="tag-browser-empty">
        <p>{t("noTags", $preferencesStore.language)}</p>
      </div>
    {:else}
      {#each groupedTags as group (group.label)}
        <section class="tag-browser-group" aria-label={group.label}>
          <h2>{group.label}</h2>
          {#each group.tags as tag (tag.id)}
            <button
              class:active={activeTagNames.includes(tag.name)}
              class="tag-browser-row"
              type="button"
              title={tag.name}
              aria-pressed={activeTagNames.includes(tag.name)}
              on:click={() => void selectTag(tag.name)}
            >
              <Tag size={16} strokeWidth={1.8} aria-hidden="true" />
              <span>{tag.name}</span>
              <small>{tag.note_count ?? 0}</small>
            </button>
          {/each}
        </section>
      {/each}
    {/if}
  </div>
</section>
