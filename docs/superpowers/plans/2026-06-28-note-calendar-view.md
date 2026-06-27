# Note Calendar View Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Memos-inspired note calendar inside the Smarticky note list.

**Architecture:** Keep the feature frontend-only. Add pure calendar helpers, extend `notesStore` with a calendar dataset that excludes date filters, and render a Svelte calendar panel from `NoteList.svelte`.

**Tech Stack:** Svelte 5, TypeScript, Vite, Vitest, existing Smarticky REST API.

## Global Constraints

- Do not add new dependencies.
- Reuse existing `created_from`, `created_to`, `updated_from`, `updated_to`, and `timezone` query parameters.
- Keep empty in-month days clickable.
- Preserve current search, tag, folder, starred, and trash scope when loading calendar activity.

---

### Task 1: Calendar Helpers

**Files:**
- Create: `web/app/src/lib/calendar/noteCalendar.ts`
- Create: `web/app/src/lib/calendar/noteCalendar.test.ts`

**Interfaces:**
- Produces `buildMonthCalendar(monthKey, counts, todayKey, selectedDate)`.
- Produces `activityCountsByDate(notes, basis, timeZone)`.
- Produces `activeCalendarFilter(searchFilters)`.
- Produces month navigation helpers.

### Task 2: Calendar Store Support

**Files:**
- Modify: `web/app/src/lib/stores/notes.ts`

**Interfaces:**
- Adds `calendarNotes`, `calendarLoading`, and `calendarError` to `NotesState`.
- Adds `loadCalendarNotes()`, `setCalendarDateFilter(date, basis)`, and `clearCalendarDateFilter()`.
- Makes existing filter-changing actions refresh calendar notes.

### Task 3: Calendar Component

**Files:**
- Create: `web/app/src/lib/components/workspace/NoteCalendar.svelte`
- Modify: `web/app/src/lib/components/workspace/NoteList.svelte`
- Modify: `web/app/src/lib/stores/preferences.ts`
- Modify: `web/app/src/lib/styles/global.css`

**Interfaces:**
- Adds a toolbar calendar toggle.
- Renders month navigation, time-basis segmented controls, date cells, and inline year overview.
- Date click applies the correct existing date filter.

### Task 4: Verification And Release

**Commands:**
- `cd web/app && npm run check`
- `cd web/app && npm test`
- `go test ./...`
- `cd web/app && npm run build`
- If local FD limits produce `EMFILE`, rerun the same build with bounded `fs.promises.readFile` concurrency.
- Commit, tag the next patch version after `v0.7.8`, and push branch plus tag.
