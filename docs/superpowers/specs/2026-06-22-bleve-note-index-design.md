# Bleve Note Index and Index Workspace Design

Date: 2026-06-22
Status: Approved

## Summary

Smarticky should use a real note index for search and expose an Index workspace inspired by the reference screenshot: a left index/category rail, a central relationship map, and a right connected-notes inspector.

The backend index is a derived data structure built with `github.com/blevesearch/bleve/v2`. SQLite/Ent remains the source of truth. The index may be rebuilt from note rows at startup or on demand, and index failures must not delete or mutate user notes.

## Goals

- Replace note body/title SQL `LIKE` search for `q` with Bleve-backed indexed search.
- Keep existing note filters: starred, trash, folder, tags, title, created date, updated date, and timezone behavior.
- Add an Index workspace that lets users browse notes by tag, folder, protection mode, and current search results.
- Render a lightweight relationship map using existing note, tag, and folder data.
- Preserve note protection boundaries: encrypted note bodies are never indexed, searched, or exposed by MCP.

## Non-Goals

- No semantic/vector/RAG search in this pass.
- No persistent graph database.
- No automatic AI relationship extraction.
- No indexing of encrypted note plaintext or `encrypted_content`.
- No folder encryption.
- No separate index table in SQLite.

## Backend Search Model

Add an internal search service:

```go
package search

type Service struct { ... }

type Document struct {
    ID             string
    UserID         int
    Title          string
    Content        string
    Tags           []string
    FolderID       string
    ProtectionMode string
    IsDeleted      bool
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

type SearchOptions struct {
    UserID       int
    Query        string
    IncludeTrash bool
    Limit        int
}
```

The index service stores only searchable derived fields. For `protection_mode="encrypted"`, the document `Content` field is always empty. Password-protected notes may index `Content`, because password mode is an access gate and not true encryption.

Bleve document IDs use note UUID strings. Index search returns ordered UUIDs; Ent applies authorization and structured filters before responses are serialized.

## Derived Data Rules

- SQLite/Ent note rows are the source of truth.
- The Bleve index can be deleted and rebuilt without losing user data.
- Startup opens the index from `<data-dir>/search.bleve`. If it does not exist, create it and rebuild from notes.
- Backend write paths update the index after successful DB writes.
- If an index update fails after a DB write, log the error and keep serving the DB result. A later rebuild can repair drift.
- Tests may use an in-memory Bleve index.

## Search Semantics

`GET /api/notes?q=...` uses Bleve for candidate IDs, then Ent for ownership, trash, folder, tag, title, and date filters.

Ordering for `q` results follows Bleve score order as far as practical after Ent filtering. If `q` is empty, keep current `updated_at DESC` ordering.

MCP search uses the same notes service and therefore follows the same protection rules:

- `none`: title and body may match.
- `password`: title and body may match, but MCP output redacts content.
- `encrypted`: title may match; body never matches.

## Index Workspace UI

Add an Index view reachable from the sidebar. It replaces the note-list pane while keeping the editor available when a note is selected.

Layout:

- Left rail: Index title, total count, groups for tags, folders, protection mode, and current search.
- Center map: SVG relationship graph generated from currently loaded notes. Notes connect to their tags, folder, and protection node. Nodes are deterministic, not physics-dependent.
- Right inspector: selected node details and connected notes. Selecting a note opens it in the editor.

The first version does not need zoom/pan persistence or a heavy graph library. It should stay responsive and usable on mobile by stacking the rail, map, and inspector.

## API and Frontend Contracts

No new public endpoint is required for the first Index workspace. The frontend uses existing note list responses plus tag/folder stores.

Frontend additions:

```ts
type WorkspaceView = "notes" | "index";
```

The notes store owns the active workspace view so Sidebar and Workspace can switch consistently.

## Security

- Search query input remains user-controlled; all DB filtering must use Ent predicates, never string-concatenated SQL.
- Do not log search queries together with note contents.
- Do not store decrypted encrypted-note content in the index, localStorage, or persistent browser storage.
- MCP must not expose protected body content through search results or generated images.

## Testing

Backend:

- Indexing a plaintext note makes title and body searchable.
- Indexing a password-protected note makes title and body searchable.
- Indexing an encrypted note makes title searchable but not body or ciphertext.
- Rebuilding the index from DB preserves the encrypted-body exclusion.
- Notes service search uses Bleve results and still enforces user ownership.

Frontend:

- Sidebar Index button switches to Index workspace.
- Index workspace renders groups, graph nodes, and inspector from loaded notes.
- Clicking a note in the inspector selects the note.
- Text does not overflow compact panels on desktop or mobile.
