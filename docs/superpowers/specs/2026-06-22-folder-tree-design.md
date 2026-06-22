# Folder Tree Design

## Goal

Add a first-class folder tree for organizing notes. Folders can be nested, notes belong to one folder at most, and both folders and notes can be starred.

## Product Model

- A folder is a user-owned resource with `id`, `name`, `parent_id`, `sort_order`, `is_starred`, and timestamps.
- Folders form a tree. The default maximum depth is 3 levels.
- Admins can configure the maximum folder depth. The backend enforces the setting and the frontend uses it to disable invalid create or move actions.
- A note may have `folder_id` or be unfiled. Existing notes remain unfiled after migration.
- Selecting a folder filters the note list to that folder. Creating a note while a folder is selected creates the note in that folder.
- The starred view shows starred folders and starred notes as separate groups.
- Trash remains a note state, not a folder. Deleting a folder is allowed only when it has no child folders and no notes.

## API Shape

- `GET /api/folders` returns the current user's folder tree as a flat ordered list with note counts.
- `POST /api/folders` creates a folder under an optional parent.
- `PUT /api/folders/:id` renames, moves, reorders, or stars a folder.
- `DELETE /api/folders/:id` deletes an empty folder.
- `GET /api/folders/settings` returns folder settings.
- `PUT /api/folders/settings` updates folder settings and is admin-only.
- `GET /api/notes?folder_id=<uuid>` filters notes by folder.
- `GET /api/notes?folder_id=unfiled` returns unfiled notes.
- `POST /api/notes` and `PUT /api/notes/:id` accept `folder_id`.

## UI Shape

- The left sidebar keeps fixed views: All notes, Starred, Trash.
- Below fixed views, it shows a folder tree with expand/collapse, note counts, star state, and compact actions.
- The note list title reflects the active folder. The new note action creates inside the active folder.
- Starred view renders folder results first, then note results.
- Settings gains an admin-only folder settings panel for maximum depth.

## Non-Goals

- No multi-folder notes.
- No drag-and-drop in the first version.
- No folder trash; deletion is hard delete for empty folders.
- No shared folder permissions in this version.
