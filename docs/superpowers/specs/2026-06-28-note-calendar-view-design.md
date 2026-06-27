# Note Calendar View Design

## Goal

Add a Memos-inspired calendar view to help users revisit notes by date without leaving the note list.

## Reference

The referenced `usememos/memos` implementation uses an activity calendar inside the explorer surface:

- The default view shows a month calendar.
- Month navigation is lightweight.
- The month label opens a year overview with twelve mini month cards.
- Calendar cells encode note count by intensity.
- In-month empty days remain clickable.
- Date clicks apply a list filter rather than opening a separate page.

## Smarticky Shape

- The first version lives inside `NoteList.svelte`, directly below the search toolbar.
- A calendar toggle in the toolbar opens or hides the calendar.
- The calendar defaults to updated-time activity because Smarticky already sorts and groups notes by `updated_at`.
- Users can switch between updated-time and created-time activity.
- Clicking a date applies the existing single-day date filters:
  - updated mode: `updated_from = date`, `updated_to = date`
  - created mode: `created_from = date`, `created_to = date`
- Clearing the calendar filter removes both created and updated date filters.
- The year overview is inline, not a modal, to keep the Svelte implementation small and mobile-friendly.

## Data

Smarticky does not yet have a statistics endpoint. The first version reuses `GET /api/notes` and loads a separate calendar dataset that keeps the active list scope but intentionally excludes date filters. That keeps the calendar useful after the user clicks a day.

The calendar dataset preserves:

- note view filter: all, starred, trash
- active folder or unfiled scope
- search text
- title keyword
- tags
- timezone

It excludes:

- `created_from`
- `created_to`
- `updated_from`
- `updated_to`

## Non-Goals

- No backend stats endpoint in this version.
- No backdated note creation.
- No route or URL state for the calendar panel.
- No dependency such as dayjs.

## Tests

- Unit-test date helpers for timezone grouping, month matrix boundaries, intensity calculation, and date-filter detection.
- Use existing Svelte check and build to cover component wiring.
