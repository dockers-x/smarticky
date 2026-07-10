# Compact Notebook Workspace Design

## Goal

Make notebook navigation and note browsing feel as direct and space-efficient as mature desktop note applications. Frequent actions should take one click, the interface should keep more notes visible, and notebook browsing should no longer cover the workspace with a large floating panel.

## Reference Model

The interaction model combines proven patterns from Apple Notes, UpNote, Evernote, and Bear:

- Primary destinations and the notebook tree share one navigation sidebar.
- The current notebook determines where a newly created note belongs.
- Notes use compact list rows rather than large bordered cards.
- Destructive and selection controls stay hidden until they are relevant.
- Desktop keeps a stable navigation/list/editor relationship; mobile presents the same hierarchy sequentially.

This is an interaction reference, not a visual clone. Smarticky keeps its existing warm paper palette, typography, brand color, folder hierarchy, filters, calendar, tags, drag-and-drop, and note editing capabilities.

## Information Architecture

The desktop workspace remains a three-pane application:

1. A 224px navigation sidebar containing Smarticky, primary views, notebooks, tags, and utilities.
2. A 320px note list containing the current context, search, filters, selection state, and notes.
3. The existing flexible editor pane.

When the sidebar is compact, it becomes a 56px icon rail. The note list remains present so compact mode increases editor space without hiding the user's current note context.

The notebook browser is no longer a floating surface during ordinary browsing. The existing folder picker remains available only for contextual tasks that require choosing a destination, such as moving selected notes or explicitly choosing a notebook during a special flow.

## Navigation Sidebar

### Primary views

Keep these destinations at the top:

- All notes
- Index
- Starred
- Trash

Notebook groups and tags become expandable sidebar sections instead of peer navigation destinations that open floating browsers.

### Notebook section

- Show a compact section heading with an expand/collapse control and a new-notebook action.
- Render the existing hierarchical folder tree inline.
- Each row shows an expand chevron when needed, folder icon, truncated name, and direct note count.
- Clicking a row immediately opens that notebook in the note list.
- The active notebook uses a quiet filled background and a brand-colored indicator.
- Row actions appear on hover or keyboard focus and retain create-child, star, rename, and delete behavior.
- Dragging folders and dropping notes on folders continue to work.
- Unfiled is the final notebook row.
- Search is not permanently visible. A section action opens the existing searchable folder picker when users need to locate a notebook in a large hierarchy.

### Tag section

- Show a collapsed summary by default to avoid competing with notebooks.
- Expanding the section shows frequently used or available tags with note counts.
- A search action opens the existing searchable tag browser when the complete tag list is needed.
- Clicking a tag applies the existing tag filter and updates the note-list context.

### Compact sidebar

- Compact mode is a 56px icon rail with tooltips.
- Notebook and tag sections are represented by their section actions; activating them opens the searchable picker.
- Frequently repeated keyboard-driven navigation is instantaneous and has no entrance animation.

## Note Creation

The primary new-note button becomes a one-step action:

- In a notebook, create the note in that notebook.
- In Unfiled, create an unfiled note.
- In All notes, Starred, Trash, Index, or a tag filter, create an unfiled note.
- The existing keyboard shortcut follows the same rule.
- After creation, select the new note and focus the editor as today.

Choosing a notebook before every new note is removed from the default path. Moving or filing a note remains available after capture.

## Note List

### Header and toolbar

- Use a compact header with the current context title and note count on one line.
- When a nested notebook is active, show a concise breadcrumb beneath the title only when it adds useful hierarchy.
- Remove the redundant notebook-groups back button because notebooks are directly accessible in the sidebar.
- Keep the new-note action at the top-right.
- Use a 36px search field.
- Calendar and advanced filters become icon buttons with accessible labels and active-count badges.
- Empty-trash remains text-labelled because it is destructive and uncommon.

### Note rows

- Replace bordered cards with 72–84px flat rows separated by subtle dividers.
- Show a one-line title, up to two compact preview lines, and a small metadata line.
- Keep the timestamp aligned consistently and shorten it when the locale permits.
- Show at most two tags in the row; summarize additional tags with a count.
- Hide selection, star, and trash controls until hover, keyboard focus, active selection mode, or touch-specific disclosure.
- The selected note uses a quiet tinted background and a 2px brand indicator rather than a card border and shadow.
- Preserve dragging, multi-selection, starring, deletion, protected-note behavior, and date grouping.

### Selection mode

- The first explicit selection reveals checkboxes for all visible rows.
- Keep the compact selection toolbar for select-all, clear, move, and delete.
- Moving notes opens the destination picker without changing the current browse context until the operation succeeds.

## Contextual Pickers

The folder and tag browser components remain reusable, but their role changes:

- Desktop contextual pickers are narrow anchored surfaces, not tall workspace-covering cards.
- Mobile contextual pickers remain full-width sheets because inline desktop navigation does not fit narrow screens.
- Popovers originate from their trigger and close on outside click or Escape.
- Folder search, hierarchy expansion, destination selection, folder management, and tag search remain intact.

## Mobile Behavior

- Keep a single-column navigation → note list → editor flow.
- Notebook and tag actions open full-height navigation sheets.
- Note rows remain compact but retain at least 44px touch targets for disclosed actions.
- Opening a note hides list navigation as today; the editor back action returns to the same list context and scroll position.
- No desktop hover-only action is required to complete a task on touch devices.

## Motion and Feedback

Apply the Emil Kowalski interaction principles:

- Do not animate note selection, notebook switching, keyboard shortcuts, or sidebar navigation.
- Use only color/opacity transitions of 120–160ms for frequent hover and focus feedback.
- Use `transform: scale(0.97)` for press feedback on compact icon buttons.
- Use a 160–200ms custom ease-out for occasional anchored pickers.
- Pickers scale from at least `0.97`, never from zero, and use the trigger-aware transform origin.
- Animate only transform and opacity.
- Respect `prefers-reduced-motion` by removing positional movement while retaining useful color and opacity feedback.

## Accessibility

- Maintain semantic navigation, tree, list, and dialog labels.
- Preserve visible focus rings and keyboard access to every hidden-on-hover action.
- Use `aria-expanded`, `aria-current` or `aria-pressed`, and descriptive action labels consistently.
- Tooltips are required for compact-sidebar and icon-only toolbar actions.
- Counts and visual active indicators must not be the only way state is communicated.

## Data and State

No backend schema or API change is required.

- Continue using `foldersStore` for tree construction, active folder, expansion state, updates, and drag behavior.
- Continue using `notesStore` for filters, folder scope, tag scope, selection, and creation.
- Add only the minimum preference state needed for notebook/tag section expansion if persistence materially improves the experience; otherwise use sensible session defaults.
- Keep folder and tag browser state separate from ordinary inline navigation so contextual pickers do not mark a navigation destination active merely because they are open.

## Error Handling

- Existing create, move, rename, delete, and load errors continue to use the notification system.
- Failed note creation leaves the current context unchanged.
- Failed moves preserve the selection so the user can retry.
- Loading and empty states occupy the list or sidebar section without replacing the rest of the workspace.

## Non-Goals

- No backend notebook model changes.
- No new routing or URL state.
- No resizable pane implementation in this release.
- No command palette or global quick switcher.
- No redesign of the editor, settings panel, index graph, authentication, or color system.
- No attempt to copy another product's branding or exact appearance.

## Verification

- Add or update focused component/store tests for the changed note-creation context and navigation state where practical.
- Run `npm run check`, `npm test`, and `npm run build` in `web/app`.
- Run relevant Go tests and `go test ./...` because built frontend assets are embedded and validated by Go tests.
- Dogfood desktop widths around 1440px and 1024px plus mobile widths around 390px.
- Verify keyboard navigation, Escape behavior, touch access, dark theme, English and Chinese labels, folder drag/drop, note selection/move, empty folders, long notebook names, and long note titles.

## Acceptance Criteria

- Opening a notebook from the normal desktop workspace requires one click and no overlay.
- Creating a note requires one click and uses the current notebook when one is active.
- A 900px-tall desktop viewport shows materially more notes than the current large-card layout.
- Notebook rows, note rows, and toolbars remain usable with long localized labels.
- Selection, move, drag/drop, tag filtering, calendar, advanced filters, compact sidebar, and mobile navigation remain functional.
- Frequent navigation feels immediate; no high-frequency action waits for an animation.
- The release is committed and tagged as the next patch version after `v0.7.13`.
