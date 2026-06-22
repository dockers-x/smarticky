# Bleve Note Index Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Bleve-backed note search and an Index workspace for browsing note relationships.

**Architecture:** SQLite/Ent remains the source of truth. A rebuildable Bleve index in `<data-dir>/search.bleve` provides candidate note IDs for text search, while Ent still enforces ownership and structured filters. The frontend Index workspace renders note, tag, folder, protection, and backlink relationships with Cytoscape.js.

**Tech Stack:** Go, Echo, Ent, SQLite, `github.com/blevesearch/bleve/v2`, Svelte 5, Cytoscape.js, Vitest-free Svelte check.

## Global Constraints

- SQLite/Ent note rows are the source of truth.
- The Bleve index is derived data and may be rebuilt without deleting user notes.
- Encrypted note bodies must not be indexed.
- Encrypted note `encrypted_content` must not be indexed.
- Password-protected note bodies may be indexed but must still be redacted in protected API/MCP responses.
- Existing note filters must keep working with `q`: starred, trash, folder, tags, title, created date, updated date, and timezone.
- MCP must not expose encrypted or password-protected note body content.
- Do not add folder encryption.
- Do not add a graph database or vector search.

---

## File Structure

- `go.mod`, `go.sum`: add `github.com/blevesearch/bleve/v2`.
- `internal/search/index.go`: own Bleve mapping, open/create, memory index, document conversion, rebuild, index, delete, search.
- `internal/search/index_test.go`: cover plaintext, password, encrypted, and rebuild behavior.
- `internal/notes/service.go`: optionally consume search service for `List` queries.
- `internal/notes/service_test.go`: cover Bleve-backed search ownership and encrypted-body exclusion.
- `internal/handler/handler.go`: construct notes service with optional search service.
- `internal/handler/note.go`: use search service for API `q` search and update/delete index after writes.
- `internal/handler/note_test.go`: cover handler search using indexed content after create/update.
- `cmd/server/main.go`: open `<data-dir>/search.bleve`, rebuild at startup, and pass it into the handler.
- `web/app/src/lib/stores/notes.ts`: add `workspaceView` and `setWorkspaceView`.
- `web/app/src/lib/components/workspace/Sidebar.svelte`: add Index navigation item.
- `web/app/src/lib/components/workspace/Workspace.svelte`: switch between `NoteList` and `IndexView`.
- `web/app/src/lib/components/workspace/IndexView.svelte`: new Index workspace.
- `web/app/src/lib/stores/preferences.ts`: add Index labels.
- `web/app/src/lib/styles/global.css`: add Index workspace CSS.

## Task 1: Bleve Search Index Backend

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `internal/search/index.go`
- Create: `internal/search/index_test.go`
- Modify: `internal/notes/service.go`
- Modify: `internal/notes/service_test.go`
- Modify: `internal/handler/handler.go`
- Modify: `internal/handler/note.go`
- Modify: `internal/handler/note_test.go`
- Modify: `cmd/server/main.go`

**Interfaces:**
- Produces:
  - `package internal/search`
  - `type Service`
  - `type Document`
  - `type SearchOptions`
  - `func Open(path string) (*Service, error)`
  - `func NewMemory() (*Service, error)`
  - `func (s *Service) Rebuild(ctx context.Context, client *ent.Client) error`
  - `func (s *Service) IndexNote(ctx context.Context, row *ent.Note) error`
  - `func (s *Service) DeleteNote(id uuid.UUID) error`
  - `func (s *Service) Search(ctx context.Context, opts SearchOptions) ([]uuid.UUID, error)`
  - `func NewService(client *ent.Client, searchService *search.Service) *Service`
  - `func NewHandlerWithSearch(client *ent.Client, fs *storage.FileSystem, searchService *search.Service) *Handler`
- Consumes:
  - Ent generated note fields from the protection tasks.
  - Existing handler note write paths.

- [ ] **Step 1: Add Bleve dependency**

Run:

```bash
go get github.com/blevesearch/bleve/v2@v2.6.0
```

Expected: `go.mod` and `go.sum` update.

- [ ] **Step 2: Write failing index tests**

Create `internal/search/index_test.go` with tests that require these behaviors:

```go
func TestSearchIndexesPlainAndPasswordContentButNotEncryptedContent(t *testing.T) {
    ctx := context.Background()
    client := enttest.Open(t, "sqlite3", "file:TestSearchIndexesPlainAndPasswordContentButNotEncryptedContent?mode=memory&cache=shared&_pragma=foreign_keys(1)")
    defer client.Close()

    owner := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)
    plain := client.Note.Create().SetTitle("Plain").SetContent("needle public").SetUserID(owner.ID).SaveX(ctx)
    locked := client.Note.Create().
        SetTitle("Locked").
        SetContent("needle gated").
        SetProtectionMode(note.ProtectionModePassword).
        SetProtectionPasswordHash("hash").
        SetUserID(owner.ID).
        SaveX(ctx)
    encrypted := client.Note.Create().
        SetTitle("Encrypted Needle Title").
        SetContent("").
        SetProtectionMode(note.ProtectionModeEncrypted).
        SetEncryptedContent("needle ciphertext").
        SetEncryptionAlg("aes-gcm").
        SetEncryptionKdf("pbkdf2-sha256:310000").
        SetEncryptionSalt("salt").
        SetEncryptionNonce("nonce").
        SetUserID(owner.ID).
        SaveX(ctx)

    svc, err := NewMemory()
    if err != nil {
        t.Fatalf("NewMemory: %v", err)
    }
    for _, row := range []*ent.Note{plain, locked, encrypted} {
        if err := svc.IndexNote(ctx, row); err != nil {
            t.Fatalf("IndexNote: %v", err)
        }
    }

    bodyMatches, err := svc.Search(ctx, SearchOptions{UserID: owner.ID, Query: "needle", Limit: 10})
    if err != nil {
        t.Fatalf("Search body: %v", err)
    }
    if slices.Contains(bodyMatches, encrypted.ID) {
        t.Fatalf("encrypted body/ciphertext must not be searchable, got %v", bodyMatches)
    }
    if !slices.Contains(bodyMatches, plain.ID) || !slices.Contains(bodyMatches, locked.ID) {
        t.Fatalf("expected plain and password notes to match body, got %v", bodyMatches)
    }

    titleMatches, err := svc.Search(ctx, SearchOptions{UserID: owner.ID, Query: "Encrypted", Limit: 10})
    if err != nil {
        t.Fatalf("Search title: %v", err)
    }
    if !slices.Contains(titleMatches, encrypted.ID) {
        t.Fatalf("encrypted title should be searchable, got %v", titleMatches)
    }
}
```

Add a second test:

```go
func TestSearchRebuildUsesDatabaseRows(t *testing.T)
```

It creates notes in Ent, calls `svc.Rebuild(ctx, client)`, then verifies title/body search works and encrypted content still does not match.

- [ ] **Step 3: Run failing search tests**

Run:

```bash
go test ./internal/search -count=1
```

Expected: FAIL because `internal/search` does not exist yet.

- [ ] **Step 4: Implement `internal/search/index.go`**

Implement the interfaces from this task. Required behavior:

- `NewMemory` creates a Bleve in-memory index.
- `Open` opens an existing index path or creates a new one.
- The mapping indexes `id`, `user_id`, `title`, `content`, `tags`, `folder_id`, `protection_mode`, `is_deleted`, `created_at`, and `updated_at`.
- Use a CJK-capable analyzer when available from Bleve; otherwise use Bleve default text mapping. Keep the implementation compiling with Bleve v2.6.0.
- `IndexNote` loads tags and folder for the row, builds a document, and indexes empty `Content` for encrypted notes.
- `Rebuild` deletes existing indexed notes by recreating the index when possible or deleting/re-indexing all rows for in-memory service.
- `Search` builds a conjunction query for user ID, trash state, and text query. It returns ordered UUIDs and ignores invalid UUID hits.

- [ ] **Step 5: Wire notes service search**

Modify `internal/notes/service.go`:

- Add `search *search.Service` to `Service`.
- Change `NewService(client *ent.Client)` to a variadic-compatible signature:

```go
func NewService(client *ent.Client, searchService ...*search.Service) *Service
```

- In `List`, when `opts.Query` is non-empty and a search service exists, call `Search`, then apply `note.IDIn(ids...)` plus existing Ent filters. Preserve Bleve result order in the returned slice.
- If the search service errors, fall back to the existing Ent title/body predicates while preserving encrypted-body exclusion.

- [ ] **Step 6: Wire handler search and index writes**

Modify `internal/handler/handler.go`:

- Add `search *search.Service` to `Handler`.
- Add `NewHandlerWithSearch(client, fs, searchService)` and make `NewHandler` call it with nil search.
- Pass the search service into `notes.NewService(client, searchService)`.

Modify `internal/handler/note.go`:

- In `ListNotes`, when `q` is non-empty and `h.search` is non-nil, get candidate IDs from Bleve and add `note.IDIn(ids...)`. Preserve result order after Ent filters.
- After successful create/update, call `h.search.IndexNote(ctx, n)` if search exists.
- After permanent delete, call `h.search.DeleteNote(id)` if search exists.
- After empty trash, rebuild the index if search exists.
- Index sync errors should be logged or ignored without rolling back a successful DB write.

- [ ] **Step 7: Wire production startup**

Modify `cmd/server/main.go`:

- Open `filepath.Join(dataDir, "search.bleve")`.
- If open succeeds, call `Rebuild(context.Background(), client)` after schema migration.
- Pass the service to `handler.NewHandlerWithSearch(client, fs, searchService)`.
- If search setup fails, log a warning and continue without indexed search.

- [ ] **Step 8: Add service/handler tests**

Add tests proving:

- `internal/notes.Service` search with an injected memory index returns only the current user's matching notes.
- Encrypted note body/ciphertext does not match service search.
- Handler `GET /api/notes?q=...` can find a note by indexed body after create/update when `NewHandlerWithSearch` is used.

- [ ] **Step 9: Run backend tests**

Run:

```bash
go test ./internal/search ./internal/notes ./internal/handler ./internal/mcp -count=1
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add go.mod go.sum internal/search internal/notes internal/handler cmd/server/main.go
git commit -m "Add Bleve note search index"
```

## Task 2: Frontend Index Workspace

**Files:**
- Modify: `web/app/src/lib/stores/notes.ts`
- Modify: `web/app/src/lib/components/workspace/Sidebar.svelte`
- Modify: `web/app/src/lib/components/workspace/Workspace.svelte`
- Create: `web/app/src/lib/components/workspace/IndexView.svelte`
- Modify: `web/app/src/lib/stores/preferences.ts`
- Modify: `web/app/src/lib/styles/global.css`

**Interfaces:**
- Produces:
  - `type WorkspaceView = "notes" | "index"`
  - `workspaceView` in notes store state
  - `notesStore.setWorkspaceView(view: WorkspaceView): Promise<void>`
  - `IndexView.svelte`
- Consumes:
  - `Note.protection_mode`
  - `Note.content_redacted`
  - Existing `foldersStore` and `tagsStore`

- [ ] **Step 1: Update notes store workspace view**

Modify `web/app/src/lib/stores/notes.ts`:

```ts
export type WorkspaceView = "notes" | "index";
```

Add `workspaceView: WorkspaceView` to `NotesState`, default `"notes"`, and implement:

```ts
async setWorkspaceView(view: WorkspaceView) {
  update((state) => ({ ...state, workspaceView: view, folderBrowserOpen: false }));
  if (view === "index") await load();
}
```

- [ ] **Step 2: Add Sidebar Index item**

Modify `Sidebar.svelte`:

- Import `Network` from `@lucide/svelte`.
- Add an Index button below All Notes.
- It sets `notesStore.setWorkspaceView("index")`.
- Existing note/folder filters set `workspaceView` back to `"notes"`.

- [ ] **Step 3: Create `IndexView.svelte`**

Create a Svelte component that:

- Loads tags on mount.
- Builds groups from `$notesStore.notes`, `$tagsStore`, and `$foldersStore.folders`.
- Shows a left `.index-sidebar` with group counts.
- Shows a center `.index-graph` Cytoscape canvas.
- Shows a right `.index-inspector` with selected node and connected notes.
- Clicking a note in the inspector calls `notesStore.select(note)`.
- The graph is deterministic: position the root in the center, note nodes on a large ring, tag/folder/protection nodes on smaller side rings.
- Do not use `innerHTML`.

The component should define local node/link types:

```ts
type IndexNodeType = "root" | "note" | "tag" | "folder" | "protection";
interface IndexNode {
  id: string;
  type: IndexNodeType;
  label: string;
  count: number;
  note?: Note;
  x: number;
  y: number;
}
interface IndexLink {
  source: string;
  target: string;
}
```

- [ ] **Step 4: Switch Workspace view**

Modify `Workspace.svelte`:

- Import `IndexView`.
- Render `IndexView` instead of `NoteList` when `$notesStore.workspaceView === "index"`.
- Keep `EditorPane note={$notesStore.selected}` unchanged.

- [ ] **Step 5: Add localized labels**

Add Chinese and English strings:

```ts
index: "索引" / "Index"
indexSearch: "搜索索引" / "Search index"
indexAllNotes: "全部索引" / "All indexed"
indexTags: "标签" / "Tags"
indexFolders: "笔记本组" / "Notebook groups"
indexProtection: "保护状态" / "Protection"
indexConnections: "连接" / "connections"
indexNoSelection: "选择一个索引节点" / "Select an index node"
indexOpenNote: "打开笔记" / "Open note"
```

- [ ] **Step 6: Add CSS**

Add CSS to `global.css` for:

- `.index-view`
- `.index-sidebar`
- `.index-graph`
- `.index-inspector`
- `.index-node`
- `.index-link`

Use existing color tokens. Keep cards at 8px radius or less. Do not use decorative gradients or orbs. Ensure the layout stacks cleanly under `max-width: 960px`.

- [ ] **Step 7: Run frontend verification**

Run:

```bash
cd web/app
mise exec -- npm run check
mise exec -- npm run build
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add web/app/src/lib/stores/notes.ts web/app/src/lib/components/workspace/Sidebar.svelte web/app/src/lib/components/workspace/Workspace.svelte web/app/src/lib/components/workspace/IndexView.svelte web/app/src/lib/stores/preferences.ts web/app/src/lib/styles/global.css
git commit -m "Add index workspace"
```
