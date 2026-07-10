# Compact Notebook Workspace Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the floating-first notebook workflow with an inline desktop notebook tree, one-click contextual note creation, and a materially denser note list.

**Architecture:** Reuse `FolderBrowserPane.svelte` as the single owner of folder-tree behavior by adding an inline sidebar variant, then compose it into `Sidebar.svelte`. Keep the existing browser overlay for contextual search/move/create flows and mobile. Refine `NoteList.svelte`, `NoteCard.svelte`, and `global.css` without changing backend APIs or editor behavior.

**Tech Stack:** Svelte 5, TypeScript, Svelte stores, Lucide Svelte, CSS, Vitest, Go embedded assets.

## Global Constraints

- No backend schema or API changes.
- Desktop remains a navigation sidebar, note list, and flexible editor.
- Notebook switching and note selection have no entrance animation.
- UI motion stays below 200ms and animates only transform, opacity, or color.
- Touch actions remain available without hover.
- Preserve folder hierarchy management, drag/drop, multi-selection, move, tag filters, calendar, filters, themes, and localization.
- Release as the next patch version after `v0.7.13`.

---

### Task 1: Lock Down Contextual Note Creation

**Files:**
- Modify: `web/app/src/lib/stores/notes.ts`
- Create: `web/app/src/lib/stores/notes.test.ts`

**Interfaces:**
- Produces: `resolveNewNoteFolderID(filter: NoteFilter, folderID: string | null, requestedFolderID?: string | null): string | null`
- Consumed by: `notesStore.create()` and the one-click button in Task 3.

- [ ] **Step 1: Write the failing folder-resolution tests**

```ts
import { describe, expect, it } from "vitest";
import { resolveNewNoteFolderID } from "./notes";

describe("resolveNewNoteFolderID", () => {
  it("uses the active folder in the all-notes scope", () => {
    expect(resolveNewNoteFolderID("all", "folder-1")).toBe("folder-1");
  });

  it("creates unfiled notes outside a folder scope", () => {
    expect(resolveNewNoteFolderID("starred", null)).toBeNull();
    expect(resolveNewNoteFolderID("trash", null)).toBeNull();
  });

  it("honors an explicit destination", () => {
    expect(resolveNewNoteFolderID("starred", null, "folder-2")).toBe("folder-2");
    expect(resolveNewNoteFolderID("all", "folder-1", null)).toBeNull();
  });
});
```

- [ ] **Step 2: Run the focused test and confirm it fails**

Run: `cd web/app && npm test -- src/lib/stores/notes.test.ts`

Expected: FAIL because `resolveNewNoteFolderID` is not exported.

- [ ] **Step 3: Add the resolver and use it from `create()`**

```ts
export function resolveNewNoteFolderID(
  filter: NoteFilter,
  folderID: string | null,
  requestedFolderID?: string | null,
): string | null {
  if (requestedFolderID !== undefined) return requestedFolderID;
  return filter === "all" ? folderID : null;
}

// Inside notesStore.create:
const targetFolderID = resolveNewNoteFolderID(
  state.filter,
  state.folderID,
  folderID,
);
```

- [ ] **Step 4: Run the focused test**

Run: `cd web/app && npm test -- src/lib/stores/notes.test.ts`

Expected: PASS.

### Task 2: Embed Notebook and Tag Navigation in the Sidebar

**Files:**
- Modify: `web/app/src/lib/components/workspace/FolderBrowserPane.svelte`
- Modify: `web/app/src/lib/components/workspace/Sidebar.svelte`
- Modify: `web/app/src/lib/components/workspace/Workspace.svelte`
- Modify: `web/app/src/lib/styles/global.css`

**Interfaces:**
- `FolderBrowserPane` gains `export let variant: "browser" | "sidebar" = "browser"`.
- `Sidebar` continues to consume `onOpenFolderBrowser` and `onOpenTagBrowser` for searchable contextual pickers.

- [ ] **Step 1: Add the inline folder-tree variant**

Add the variant prop and conditional browser chrome:

```svelte
export let variant: "browser" | "sidebar" = "browser";
$: sidebarVariant = variant === "sidebar";
```

Add `class:folder-browser-pane--sidebar={sidebarVariant}` to the existing root section. Insert `{#if !sidebarVariant}` immediately before the existing `folder-browser-header` and close it immediately after the existing `folder-browser-search` label. Change the All notes condition to:

```svelte
{#if mode !== "create" && !sidebarVariant}
```

This keeps row activation, hierarchy expansion, folder actions, and drag/drop in the shared component while removing browser-only chrome from the inline variant.

- [ ] **Step 2: Restructure the sidebar navigation**

Implement:

```svelte
<nav class="sidebar__nav">
  {#each filters as filter (filter.id)}
    <button type="button" on:click={() => void selectFilter(filter.id)}>
      <svelte:component this={filter.icon} size={17} strokeWidth={1.8} />
      <span class="sidebar__label">{filter.label}</span>
    </button>
  {/each}
</nav>

{#if !$preferencesStore.sidebarCompact}
  <section class="sidebar-section sidebar-section--notebooks">
    <header class="sidebar-section__header">
      <button class="sidebar-section__toggle" type="button" on:click={() => (notebooksExpanded = !notebooksExpanded)}>
        <ChevronDown size={14} strokeWidth={2} />
        <span>{t("notebookGroups", $preferencesStore.language)}</span>
      </button>
      <button class="sidebar-section__action" type="button" aria-label={t("searchNotebookGroups", $preferencesStore.language)} on:click={onOpenFolderBrowser}>
        <Search size={14} strokeWidth={2} />
      </button>
    </header>
    {#if notebooksExpanded}
      <FolderBrowserPane variant="sidebar" />
    {/if}
  </section>

  <section class="sidebar-section sidebar-section--tags">
    <header class="sidebar-section__header">
      <button class="sidebar-section__toggle" type="button" on:click={() => (tagsExpanded = !tagsExpanded)}>
        <ChevronRight size={14} strokeWidth={2} />
        <span>{t("tags", $preferencesStore.language)}</span>
      </button>
      <button class="sidebar-section__action" type="button" aria-label={t("searchTags", $preferencesStore.language)} on:click={onOpenTagBrowser}>
        <Search size={14} strokeWidth={2} />
      </button>
    </header>
  </section>
{:else}
  <button class="sidebar__compact-destination" type="button" aria-label={t("notebookGroups", $preferencesStore.language)} on:click={onOpenFolderBrowser}>
    <Folder size={17} strokeWidth={1.8} />
  </button>
  <button class="sidebar__compact-destination" type="button" aria-label={t("tags", $preferencesStore.language)} on:click={onOpenTagBrowser}>
    <Tag size={17} strokeWidth={1.8} />
  </button>
{/if}
```

Load `tagsStore`, show direct counts, and cap inline tags to a practical list while leaving full search in the contextual browser.

- [ ] **Step 3: Update workspace geometry and sidebar density**

Use these target values in `global.css`:

```css
.workspace { grid-template-columns: 224px 320px minmax(0, 1fr); }
.workspace:has(.sidebar.compact) { grid-template-columns: 56px 320px minmax(0, 1fr); }
.sidebar { padding: 12px 10px; }
.sidebar.compact { padding: 10px 6px; }
.sidebar__nav { gap: 2px; }
.sidebar__nav > button { min-height: 36px; }
```

Add scroll containment to the notebook section and ensure utilities remain reachable at the bottom.

- [ ] **Step 4: Run Svelte validation**

Run: `cd web/app && npm run check`

Expected: zero Svelte or TypeScript errors.

### Task 3: Make the Note List Compact and One-Click

**Files:**
- Modify: `web/app/src/lib/components/workspace/NoteList.svelte`
- Modify: `web/app/src/lib/components/workspace/NoteCard.svelte`
- Modify: `web/app/src/lib/styles/global.css`

**Interfaces:**
- `NoteCard` gains `selectionMode: boolean`.
- `NoteList` calls `notesStore.create()` directly for its primary new-note action.
- `NoteList` keeps `onOpenFolderBrowser("move")` only for explicit move operations.

- [ ] **Step 1: Replace folder-picker creation with direct creation**

```ts
async function createNote(): Promise<void> {
  try {
    await notesStore.create();
  } catch {
    notify(t("createNoteFailed", $preferencesStore.language), "error");
  }
}
```

Wire the new-note button to `createNote`, remove the redundant notebook-groups back button, and retain a concise breadcrumb for nested folders.

- [ ] **Step 2: Collapse toolbar labels into accessible icon actions**

Keep the search field visible, use icon-only calendar/filter buttons with `aria-label` and `title`, and retain active-count badges. Empty Trash remains labelled.

- [ ] **Step 3: Add explicit selection mode to note rows**

```svelte
<NoteCard
  {note}
  active={$notesStore.selected?.id === note.id}
  selected={selectedNoteIDs.includes(note.id)}
  selectionMode={selectedCount > 0}
  dragNoteIDs={selectedNoteIDs}
  onToggleSelected={toggleSelected}
  onToggleStar={toggleNoteStar}
  onDelete={deleteNoteFromList}
/>
```

In `NoteCard.svelte`, add `class:selection-mode={selectionMode}` and show the checkbox on selection mode, row hover, or focus-within. Limit visible tags to two.

- [ ] **Step 4: Replace card styling with dense rows**

```css
.note-card-list { gap: 0; margin-top: 8px; }
.note-group { gap: 0; }
.note-card {
  min-height: 78px;
  gap: 5px;
  border: 0;
  border-bottom: 1px solid var(--color-divider);
  border-radius: 0;
  background: transparent;
  padding: 10px 8px;
  box-shadow: none;
}
.note-card.active { background: color-mix(in srgb, var(--color-brand) 8%, transparent); }
.note-card__title { margin: 0 0 3px; font-size: 14px; line-height: 1.35; -webkit-line-clamp: 1; }
.note-card__preview { font-size: 12px; line-height: 1.4; -webkit-line-clamp: 2; }
```

Use opacity transitions no longer than 140ms for hover-revealed actions; do not animate active-row selection.

- [ ] **Step 5: Run focused tests and Svelte validation**

Run: `cd web/app && npm test -- src/lib/stores/notes.test.ts && npm run check`

Expected: all tests pass and zero Svelte errors.

### Task 4: Refine Contextual Pickers and Responsive Behavior

**Files:**
- Modify: `web/app/src/lib/components/workspace/Workspace.svelte`
- Modify: `web/app/src/lib/styles/global.css`

**Interfaces:**
- Existing `folderBrowserMode` remains the source of contextual picker mode.
- Mobile continues to use the existing full-width browser overlay.

- [ ] **Step 1: Make desktop browser overlays compact and trigger-aware**

```css
.workspace-browser-overlay {
  top: 58px;
  bottom: auto;
  left: 214px;
  width: min(340px, calc(100vw - 236px));
  max-height: min(620px, calc(100dvh - 76px));
  transform-origin: left top;
  transition: opacity 180ms var(--ease-out), transform 180ms var(--ease-out);
}

@starting-style {
  .workspace-browser-overlay { opacity: 0; transform: scale(0.97); }
}
```

Keep the mobile media query as a full-width sheet and ensure reduced motion removes scale.

- [ ] **Step 2: Verify desktop and mobile interaction manually**

Use the local server and check:

- 1440×900: inline notebooks, dense note rows, editor space.
- 1024×768: no clipped sidebar or toolbar.
- 390×844: notebook/tag sheets, note actions, editor return flow.
- Light/dark themes and Chinese/English labels.

- [ ] **Step 3: Run the complete frontend suite**

Run: `cd web/app && npm run check && npm test && npm run build`

Expected: all commands exit zero and production assets are regenerated under `web/static/app`.

### Task 5: Release Verification and Delivery

**Files:**
- Modify: generated files under `web/static/app/`
- Modify: `CHANGELOG.md` if it uses per-release entries.

**Interfaces:** None.

- [ ] **Step 1: Review the final diff**

Run: `git diff --check && git status --short && git diff --stat`

Expected: no whitespace errors; only the compact workspace implementation, tests, docs, and generated frontend assets are present.

- [ ] **Step 2: Run repository verification**

Run: `go test ./...`

Expected: all Go packages pass, including embedded-asset validation.

- [ ] **Step 3: Commit the implementation**

```bash
git add web/app/src web/static/app CHANGELOG.md docs/superpowers/plans/2026-07-10-compact-notebook-workspace.md
git commit -m "feat: streamline notebook workspace"
```

If `CHANGELOG.md` is unchanged, omit it from `git add`.

- [ ] **Step 4: Push commits and release tag**

```bash
git push origin main
git tag -a v0.7.14 -m "v0.7.14"
git push origin v0.7.14
```

- [ ] **Step 5: Confirm remote state**

Run: `git status --short --branch && git ls-remote --tags origin refs/tags/v0.7.14`

Expected: local `main` matches `origin/main`, the worktree is clean, and the remote tag exists.
