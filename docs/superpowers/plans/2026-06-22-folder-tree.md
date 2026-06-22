# Folder Tree Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build nested folders for notes, with admin-configured depth and starred folders visible in the starred view.

**Architecture:** Add `Folder` and folder settings to Ent, expose REST endpoints from `internal/handler`, and extend existing note list filtering rather than creating a second note workflow. The frontend adds a `foldersStore`, folder API module, sidebar tree, and folder-aware note creation/update.

**Tech Stack:** Go, Ent, Echo, Svelte 5, Vite, Vitest.

## Global Constraints

- Default maximum folder depth is 3.
- Admins can configure maximum folder depth.
- Notes belong to at most one folder.
- Folders and notes both support starring.
- Starred view shows folders and notes.
- Creating a note while a folder is selected creates it in that folder.
- Deleting a folder is allowed only when it has no child folders and no notes.

---

### Task 1: Backend Folder Model And Settings

**Files:**
- Create: `ent/schema/folder.go`
- Modify: `ent/schema/note.go`
- Modify: `ent/schema/user.go`
- Modify: `ent/schema/backupconfig.go`
- Generate: `ent/**`

**Interfaces:**
- Produces: `Folder` entity with `parent`, `children`, `notes`, and `user` edges.
- Produces: `BackupConfig.folder_max_depth int`.
- Produces: `Note.folder` optional edge.

### Task 2: Folder REST API

**Files:**
- Create: `internal/handler/folder.go`
- Create: `internal/handler/folder_test.go`
- Modify: `internal/handler/note.go`
- Modify: `cmd/server/main.go`

**Interfaces:**
- Produces: `GET/POST/PUT/DELETE /api/folders`.
- Produces: `GET/PUT /api/folders/settings`.
- Extends notes API with `folder_id`.

### Task 3: Frontend Folder State

**Files:**
- Create: `web/app/src/lib/api/folders.ts`
- Create: `web/app/src/lib/stores/folders.ts`
- Modify: `web/app/src/lib/api/types.ts`
- Modify: `web/app/src/lib/stores/notes.ts`

**Interfaces:**
- Produces: `foldersStore` with tree loading, active folder selection, CRUD helpers, and settings helpers.
- Extends `notesStore.create()` to use the active folder.

### Task 4: Sidebar And Starred View UI

**Files:**
- Modify: `web/app/src/lib/components/workspace/Sidebar.svelte`
- Modify: `web/app/src/lib/components/workspace/NoteList.svelte`
- Modify: `web/app/src/lib/styles/global.css`
- Modify: `web/app/src/lib/stores/preferences.ts`

**Interfaces:**
- Produces: expandable folder tree in sidebar.
- Produces: starred folder group in Starred view.

### Task 5: Editor Folder Move And Admin Settings

**Files:**
- Modify: `web/app/src/lib/components/editor/EditorPane.svelte`
- Create: `web/app/src/lib/components/settings/FolderSettingsPanel.svelte`
- Modify: `web/app/src/lib/components/settings/ToolsPanel.svelte`
- Modify: `web/app/src/lib/stores/preferences.ts`

**Interfaces:**
- Produces: note location selector backed by folders.
- Produces: admin panel for folder max depth.

### Task 6: Verification

**Commands:**
- `go generate ./ent`
- `gofmt` on changed Go files
- `npm run build`
- `npm run check`
- `npm run test -- --run`
- `go test ./...`
- Browser smoke: create nested folders, set active folder, create note, star folder and note, verify Starred view groups both.
