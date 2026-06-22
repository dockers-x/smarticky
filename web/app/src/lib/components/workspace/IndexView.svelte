<script lang="ts">
  import { onMount } from "svelte";
  import { FileText, Folder, Network, Search, Shield, Tag } from "@lucide/svelte";
  import IndexGraphCanvas from "./IndexGraphCanvas.svelte";
  import { fetchNoteLinkGraph } from "../../api/noteLinks";
  import type {
    Folder as FolderRecord,
    Note,
    NoteLinkGraph,
    ProtectionMode,
    Tag as TagRecord,
  } from "../../api/types";
  import { foldersStore } from "../../stores/folders";
  import { notesStore } from "../../stores/notes";
  import {
    preferencesStore,
    t,
    type Language,
  } from "../../stores/preferences";
  import { tagsStore } from "../../stores/tags";

  type IndexNodeType = "root" | "note" | "tag" | "folder" | "protection" | "relation";
  type IndexLinkKind = "membership" | "backlink";

  interface IndexNode {
    id: string;
    type: IndexNodeType;
    label: string;
    count: number;
    note?: Note;
  }

  interface IndexLink {
    source: string;
    target: string;
    kind?: IndexLinkKind;
  }

  interface GroupEntry {
    id: string;
    label: string;
    notes: Note[];
  }

  interface IndexModel {
    nodes: IndexNode[];
    links: IndexLink[];
    nodeByID: Map<string, IndexNode>;
    notesByNodeID: Map<string, Note[]>;
    tagNodes: IndexNode[];
    folderNodes: IndexNode[];
    protectionNodes: IndexNode[];
    rootNode: IndexNode;
  }

  const rootID = "root";
  const relationID = "relations";
  const emptyLinkGraph: NoteLinkGraph = { nodes: [], edges: [] };
  const protectionModes: ProtectionMode[] = ["none", "password", "encrypted"];

  let selectedNodeID = rootID;
  let linkGraph = emptyLinkGraph;
  let linkGraphLoading = false;
  let linkGraphError = "";
  let linkGraphSequence = 0;
  let mounted = false;
  let lastLinkGraphSignature = "";

  onMount(() => {
    void tagsStore.load();
    mounted = true;
  });

  function nodeID(type: Exclude<IndexNodeType, "root">, id: string): string {
    return `${type}:${id}`;
  }

  function noteTitle(note: Note, language: Language): string {
    return note.title.trim() || t("untitled", language);
  }

  function normalizeProtectionMode(mode: string | undefined): ProtectionMode {
    if (mode === "password" || mode === "encrypted") return mode;
    return "none";
  }

  function protectionLabel(mode: ProtectionMode, language: Language): string {
    if (mode === "password") return t("indexProtectionPassword", language);
    if (mode === "encrypted") return t("indexProtectionEncrypted", language);
    return t("indexProtectionNone", language);
  }

  function typeLabel(type: IndexNodeType, language: Language): string {
    if (type === "relation") return t("indexRelations", language);
    if (type === "tag") return t("indexTags", language);
    if (type === "folder") return t("indexFolders", language);
    if (type === "protection") return t("indexProtection", language);
    if (type === "note") return t("notes", language);
    return t("indexAllNotes", language);
  }

  function tagIdentity(tag: TagRecord | string): { id: string; name: string } {
    if (typeof tag === "string") return { id: tag, name: tag };
    return { id: tag.id, name: tag.name };
  }

  function noteTags(note: Note): Array<{ id: string; name: string }> {
    return ((note.tags ?? []) as Array<TagRecord | string>).map(tagIdentity);
  }

  function sortedNotes(notes: Note[], language: Language): Note[] {
    return [...notes].sort((left, right) => {
      const updated = Date.parse(right.updated_at) - Date.parse(left.updated_at);
      if (updated !== 0) return updated;
      const title = noteTitle(left, language).localeCompare(noteTitle(right, language));
      if (title !== 0) return title;
      return left.id.localeCompare(right.id);
    });
  }

  function buildIndexModel(
    notes: Note[],
    folders: FolderRecord[],
    tags: TagRecord[],
    graph: NoteLinkGraph,
    language: Language,
  ): IndexModel {
    const orderedNotes = sortedNotes(notes, language);
    const noteIDSet = new Set(orderedNotes.map((note) => note.id));
    const noteByID = new Map(orderedNotes.map((note) => [note.id, note]));
    const visibleGraphEdges = graph.edges.filter(
      (edge) => noteIDSet.has(edge.source) && noteIDSet.has(edge.target),
    );
    const rootNode: IndexNode = {
      id: rootID,
      type: "root",
      label: t("indexAllNotes", language),
      count: orderedNotes.length,
    };
    const relationNode: IndexNode = {
      id: relationID,
      type: "relation",
      label: t("indexRelations", language),
      count: visibleGraphEdges.length,
    };

    const nodes: IndexNode[] = [rootNode, relationNode];
    const links: IndexLink[] = [];
    const notesByNodeID = new Map<string, Note[]>([[rootID, orderedNotes]]);
    const tagGroups = new Map<string, GroupEntry>();
    const folderGroups = new Map<string, GroupEntry>();
    const protectionGroups = new Map<string, GroupEntry>();
    const folderByID = new Map(folders.map((folder) => [folder.id, folder]));

    for (const tag of tags) {
      tagGroups.set(tag.id, { id: tag.id, label: tag.name, notes: [] });
    }

    for (const folder of folders) {
      folderGroups.set(folder.id, {
        id: folder.id,
        label: folder.name,
        notes: [],
      });
    }

    for (const mode of protectionModes) {
      protectionGroups.set(mode, {
        id: mode,
        label: protectionLabel(mode, language),
        notes: [],
      });
    }

    for (const note of orderedNotes) {
      for (const tag of noteTags(note)) {
        const group = tagGroups.get(tag.id) ?? {
          id: tag.id,
          label: tag.name,
          notes: [],
        };
        group.notes.push(note);
        tagGroups.set(tag.id, group);
      }

      const folderID = note.folder_id ?? "unfiled";
      const folderLabel =
        folderID === "unfiled"
          ? t("unfiledNotes", language)
          : folderByID.get(folderID)?.name ?? t("unknown", language);
      const folderGroup = folderGroups.get(folderID) ?? {
        id: folderID,
        label: folderLabel,
        notes: [],
      };
      folderGroup.notes.push(note);
      folderGroups.set(folderID, folderGroup);

      const protectionMode = normalizeProtectionMode(note.protection_mode);
      const protectionGroup = protectionGroups.get(protectionMode);
      protectionGroup?.notes.push(note);
    }

    const noteNodes = orderedNotes.map((note) => ({
      id: nodeID("note", note.id),
      type: "note" as const,
      label: noteTitle(note, language),
      count: 1,
      note,
    }));

    for (const node of noteNodes) {
      nodes.push(node);
      notesByNodeID.set(node.id, node.note ? [node.note] : []);
      links.push({ source: rootID, target: node.id, kind: "membership" });
    }

    const relationNoteIDs = new Set<string>();
    const noteRelationIDs = new Map<string, Set<string>>();
    for (const edge of visibleGraphEdges) {
      relationNoteIDs.add(edge.source);
      relationNoteIDs.add(edge.target);
      const sourceNodeID = nodeID("note", edge.source);
      const targetNodeID = nodeID("note", edge.target);
      links.push({ source: sourceNodeID, target: targetNodeID, kind: "backlink" });
      const sourceRelations = noteRelationIDs.get(edge.source) ?? new Set<string>();
      sourceRelations.add(edge.target);
      noteRelationIDs.set(edge.source, sourceRelations);
      const targetRelations = noteRelationIDs.get(edge.target) ?? new Set<string>();
      targetRelations.add(edge.source);
      noteRelationIDs.set(edge.target, targetRelations);
    }
    notesByNodeID.set(
      relationID,
      sortedNotes(
        [...relationNoteIDs]
          .map((id) => noteByID.get(id))
          .filter((note): note is Note => Boolean(note)),
        language,
      ),
    );
    for (const noteID of relationNoteIDs) {
      links.push({ source: relationID, target: nodeID("note", noteID), kind: "membership" });
    }
    for (const [noteID, relatedIDs] of noteRelationIDs.entries()) {
      const currentNote = noteByID.get(noteID);
      notesByNodeID.set(
        nodeID("note", noteID),
        [
          ...(currentNote ? [currentNote] : []),
          ...sortedNotes(
            [...relatedIDs]
              .map((id) => noteByID.get(id))
              .filter((note): note is Note => Boolean(note)),
            language,
          ),
        ],
      );
    }

    const tagEntries = [...tagGroups.values()].sort((left, right) => {
      if (right.notes.length !== left.notes.length) {
        return right.notes.length - left.notes.length;
      }
      return left.label.localeCompare(right.label);
    });
    const folderEntries = [...folderGroups.values()].sort((left, right) => {
      if (right.notes.length !== left.notes.length) {
        return right.notes.length - left.notes.length;
      }
      return left.label.localeCompare(right.label);
    });
    const protectionEntries = protectionModes
      .map((mode) => protectionGroups.get(mode))
      .filter((entry): entry is GroupEntry => Boolean(entry));

    const tagNodes = tagEntries.map((entry) => ({
      id: nodeID("tag", entry.id),
      type: "tag" as const,
      label: entry.label,
      count: entry.notes.length,
    }));

    const folderNodes = folderEntries.map((entry) => ({
      id: nodeID("folder", entry.id),
      type: "folder" as const,
      label: entry.label,
      count: entry.notes.length,
    }));

    const protectionNodes = protectionEntries.map((entry) => ({
      id: nodeID("protection", entry.id),
      type: "protection" as const,
      label: entry.label,
      count: entry.notes.length,
    }));

    for (const node of [...tagNodes, ...folderNodes, ...protectionNodes]) {
      nodes.push(node);
    }

    for (const entry of tagEntries) {
      const source = nodeID("tag", entry.id);
      notesByNodeID.set(source, entry.notes);
      for (const note of entry.notes) {
        links.push({ source, target: nodeID("note", note.id) });
      }
    }

    for (const entry of folderEntries) {
      const source = nodeID("folder", entry.id);
      notesByNodeID.set(source, entry.notes);
      for (const note of entry.notes) {
        links.push({ source, target: nodeID("note", note.id) });
      }
    }

    for (const entry of protectionEntries) {
      const source = nodeID("protection", entry.id);
      notesByNodeID.set(source, entry.notes);
      for (const note of entry.notes) {
        links.push({ source, target: nodeID("note", note.id) });
      }
    }

    const nodeByID = new Map(nodes.map((node) => [node.id, node]));

    return {
      nodes,
      links,
      nodeByID,
      notesByNodeID,
      tagNodes,
      folderNodes,
      protectionNodes,
      rootNode,
    };
  }

  function selectNode(nodeIDValue: string): void {
    selectedNodeID = nodeIDValue;
  }

  function openNote(note: Note): void {
    notesStore.select(note);
  }

  async function loadLinkGraph(): Promise<void> {
    const sequence = ++linkGraphSequence;
    linkGraphLoading = true;
    linkGraphError = "";
    try {
      const graph = await fetchNoteLinkGraph(false);
      if (sequence !== linkGraphSequence) return;
      linkGraph = graph;
    } catch (error) {
      if (sequence !== linkGraphSequence) return;
      linkGraph = emptyLinkGraph;
      linkGraphError =
        error instanceof Error
          ? error.message
          : t("indexRelationGraphLoadFailed", $preferencesStore.language);
    } finally {
      if (sequence === linkGraphSequence) {
        linkGraphLoading = false;
      }
    }
  }

  function formatNoteDate(note: Note, language: Language, timeZone: string): string {
    return new Intl.DateTimeFormat(language === "zh" ? "zh-CN" : "en-US", {
      month: "numeric",
      day: "numeric",
      year: "numeric",
      hour: "numeric",
      minute: "2-digit",
      timeZone,
    }).format(new Date(note.updated_at));
  }

  function folderLabel(note: Note, folders: FolderRecord[], language: Language): string {
    if (!note.folder_id) return t("unfiledNotes", language);
    return folders.find((folder) => folder.id === note.folder_id)?.name ?? t("unknown", language);
  }

  function noteMetadata(
    note: Note,
    folders: FolderRecord[],
    language: Language,
    timeZone: string,
  ): string {
    const tagNames = noteTags(note).map((tag) => tag.name).slice(0, 2);
    const parts = [
      folderLabel(note, folders, language),
      ...tagNames,
      protectionLabel(normalizeProtectionMode(note.protection_mode), language),
      formatNoteDate(note, language, timeZone),
    ];
    return parts.filter(Boolean).join(" / ");
  }

  $: indexModel = buildIndexModel(
    $notesStore.notes,
    $foldersStore.folders,
    $tagsStore,
    linkGraph,
    $preferencesStore.language,
  );
  $: noteListSignature = $notesStore.notes
    .map((note) => `${note.id}:${note.updated_at}:${note.is_deleted}`)
    .join("|");
  $: if (mounted && !$notesStore.loading && noteListSignature !== lastLinkGraphSignature) {
    lastLinkGraphSignature = noteListSignature;
    void loadLinkGraph();
  }
  $: if (!indexModel.nodeByID.has(selectedNodeID)) {
    selectedNodeID = rootID;
  }
  $: selectedNode = indexModel.nodeByID.get(selectedNodeID) ?? indexModel.rootNode;
  $: connectedNotes = indexModel.notesByNodeID.get(selectedNode.id) ?? [];
  $: protectedConnectedNotes = connectedNotes.some(
    (note) =>
      note.content_redacted ||
      normalizeProtectionMode(note.protection_mode) !== "none",
  );
  $: activeSearch = $notesStore.search.trim();
</script>

<section
  class:editor-open={Boolean($notesStore.selected)}
  class="index-view"
  aria-label={t("index", $preferencesStore.language)}
>
  <aside class="index-sidebar" aria-label={t("indexAllNotes", $preferencesStore.language)}>
    <button
      class:active={selectedNodeID === indexModel.rootNode.id}
      class="index-rail-item index-rail-item--root"
      type="button"
      on:click={() => selectNode(indexModel.rootNode.id)}
    >
      <Network size={16} strokeWidth={1.8} aria-hidden="true" />
      <span>
        <strong>{t("indexAllNotes", $preferencesStore.language)}</strong>
        <small>{indexModel.rootNode.count} {t("notes", $preferencesStore.language)}</small>
      </span>
      <b>{indexModel.rootNode.count}</b>
    </button>

    {#if activeSearch}
      <button
        class="index-rail-item"
        type="button"
        title={activeSearch}
        on:click={() => selectNode(indexModel.rootNode.id)}
      >
        <Search size={15} strokeWidth={1.8} aria-hidden="true" />
        <span>
          <strong>{t("indexActiveSearch", $preferencesStore.language)}</strong>
          <small>{activeSearch}</small>
        </span>
        <b>{indexModel.rootNode.count}</b>
      </button>
    {/if}

    <div class="index-rail-section">
      <h2>{t("indexRelations", $preferencesStore.language)}</h2>
      <div class="index-rail-list">
        <button
          class:active={selectedNodeID === relationID}
          class="index-rail-item"
          type="button"
          on:click={() => selectNode(relationID)}
        >
          <Network size={14} strokeWidth={1.8} aria-hidden="true" />
          <span><strong>{t("indexRelations", $preferencesStore.language)}</strong></span>
          <b>{indexModel.nodeByID.get(relationID)?.count ?? 0}</b>
        </button>
      </div>
    </div>

    <div class="index-rail-section">
      <h2>{t("indexTags", $preferencesStore.language)}</h2>
      <div class="index-rail-list">
        {#each indexModel.tagNodes as node (node.id)}
          <button
            class:active={selectedNodeID === node.id}
            class="index-rail-item"
            type="button"
            title={node.label}
            on:click={() => selectNode(node.id)}
          >
            <Tag size={14} strokeWidth={1.8} aria-hidden="true" />
            <span><strong>{node.label}</strong></span>
            <b>{node.count}</b>
          </button>
        {/each}
      </div>
    </div>

    <div class="index-rail-section">
      <h2>{t("indexFolders", $preferencesStore.language)}</h2>
      <div class="index-rail-list">
        {#each indexModel.folderNodes as node (node.id)}
          <button
            class:active={selectedNodeID === node.id}
            class="index-rail-item"
            type="button"
            title={node.label}
            on:click={() => selectNode(node.id)}
          >
            <Folder size={14} strokeWidth={1.8} aria-hidden="true" />
            <span><strong>{node.label}</strong></span>
            <b>{node.count}</b>
          </button>
        {/each}
      </div>
    </div>

    <div class="index-rail-section">
      <h2>{t("indexProtection", $preferencesStore.language)}</h2>
      <div class="index-rail-list">
        {#each indexModel.protectionNodes as node (node.id)}
          <button
            class:active={selectedNodeID === node.id}
            class="index-rail-item"
            type="button"
            title={node.label}
            on:click={() => selectNode(node.id)}
          >
            <Shield size={14} strokeWidth={1.8} aria-hidden="true" />
            <span><strong>{node.label}</strong></span>
            <b>{node.count}</b>
          </button>
        {/each}
      </div>
    </div>
  </aside>

  <section class="index-graph" aria-label={t("indexGraph", $preferencesStore.language)}>
    <div class="index-graph__toolbar">
      <label class="index-search">
        <Search size={16} strokeWidth={1.8} aria-hidden="true" />
        <input
          type="search"
          aria-label={t("indexSearch", $preferencesStore.language)}
          placeholder={t("indexSearch", $preferencesStore.language)}
          value={$notesStore.search}
          on:input={(event) => void notesStore.setSearch(event.currentTarget.value)}
        />
      </label>
      <span>
        {indexModel.rootNode.count} {t("notes", $preferencesStore.language)}
        ·
        {indexModel.nodeByID.get(relationID)?.count ?? 0} {t("indexRelations", $preferencesStore.language)}
      </span>
    </div>

    {#if $notesStore.error}
      <div class="index-message" role="alert">{$notesStore.error}</div>
    {:else if $notesStore.loading}
      <div class="index-message">{t("loadingNotes", $preferencesStore.language)}</div>
    {:else if indexModel.rootNode.count === 0}
      <div class="index-message">
        {activeSearch ? t("indexNoConnections", $preferencesStore.language) : t("indexEmpty", $preferencesStore.language)}
      </div>
    {:else}
      {#if linkGraphError}
        <div class="index-message index-message--overlay" role="status">
          <span>{t("indexRelationGraphLoadFailed", $preferencesStore.language)}</span>
          <button type="button" on:click={() => void loadLinkGraph()}>
            {t("refresh", $preferencesStore.language)}
          </button>
        </div>
      {:else if linkGraphLoading}
        <div class="index-message index-message--overlay" role="status">
          {t("loading", $preferencesStore.language)}
        </div>
      {/if}
      <IndexGraphCanvas
        nodes={indexModel.nodes}
        links={indexModel.links}
        {selectedNodeID}
        ariaLabel={t("indexGraph", $preferencesStore.language)}
        theme={$preferencesStore.theme}
        onSelectNode={selectNode}
      />
    {/if}
  </section>

  <aside class="index-inspector" aria-label={t("indexNoSelection", $preferencesStore.language)}>
    {#if selectedNode}
      <header>
        <span>{typeLabel(selectedNode.type, $preferencesStore.language)}</span>
        <h2 title={selectedNode.label}>{selectedNode.label}</h2>
        <p>{connectedNotes.length} {t("indexConnections", $preferencesStore.language)}</p>
      </header>

      {#if protectedConnectedNotes}
        <p class="index-inspector__notice">
          {t("indexProtectedMetadataOnly", $preferencesStore.language)}
        </p>
      {/if}

      {#if connectedNotes.length === 0}
        <div class="index-inspector__empty">
          {t("indexNoConnections", $preferencesStore.language)}
        </div>
      {:else}
        <div class="index-connected-list">
          {#each connectedNotes as note (note.id)}
            <button
              class:active={$notesStore.selected?.id === note.id}
              class="index-connected-note"
              type="button"
              title={noteTitle(note, $preferencesStore.language)}
              on:click={() => openNote(note)}
            >
              <FileText size={15} strokeWidth={1.8} aria-hidden="true" />
              <span>
                <strong>{noteTitle(note, $preferencesStore.language)}</strong>
                <small>
                  {noteMetadata(
                    note,
                    $foldersStore.folders,
                    $preferencesStore.language,
                    $preferencesStore.timeZone,
                  )}
                </small>
              </span>
            </button>
          {/each}
        </div>
      {/if}
    {:else}
      <div class="index-inspector__empty">
        {t("indexNoSelection", $preferencesStore.language)}
      </div>
    {/if}
  </aside>
</section>
