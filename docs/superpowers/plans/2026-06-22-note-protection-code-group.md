# Note Protection and Code Group Editing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add local code-group source editing and replace legacy note locking with `protection_mode: none | password | encrypted`.

**Architecture:** Code-group editing stays inside the Milkdown widget and replaces only the matching fenced block in Markdown. Note protection becomes a first-class backend field set, with password mode redacting plaintext until verification and encrypted mode storing only browser-produced ciphertext. Legacy `is_locked` / `password` fields are removed directly and old protection metadata is not migrated.

**Tech Stack:** Go, Echo, Ent, SQLite, Svelte 5, Milkdown/Crepe, Vitest, Web Crypto, Argon2id.

## Global Constraints

- Directly remove legacy `is_locked` / `password` from the Ent note schema.
- Enable `migrate.WithDropColumn` in startup migration.
- Do not backfill old `is_locked` values or old `password` hashes.
- Existing notes default to `protection_mode="none"`.
- Do not delete note `title`, `content`, `color`, folder edges, tag edges, attachments, whiteboards, timestamps, or users.
- `encrypted` mode protects note body content only.
- Folders must not support `encrypted` mode.
- If folder-level protection is added, it is access-password only and does not encrypt all notes in the folder.
- Never send encrypted-note passwords to the server.
- Do not store decrypted encrypted-note content in localStorage or persistent browser storage.
- Encrypted notes are not server-searchable by body content.
- MCP must not expose encrypted note body content.

---

## File Structure

- `web/app/src/lib/markdown/codeGroups.ts`: add pure validation and source replacement helpers for code-group blocks.
- `web/app/src/lib/markdown/editorCodeGroups.ts`: replace global source-mode callback with inline widget source editor.
- `web/app/src/lib/components/editor/MarkdownEditor.svelte`: wire code-group source replacement into Milkdown `replaceAll` and normal `onChange`.
- `web/app/src/lib/markdown/render.test.ts`: add pure code-group replacement coverage.
- `ent/schema/note.go`: add protection fields, remove legacy `password` / `is_locked`.
- `cmd/server/main.go`: enable `migrate.WithDropColumn`.
- Generated `ent/*`: regenerate after schema change with `go generate ./ent`.
- `internal/handler/note.go`: update API request/response structs, redaction, password verification, and strict JSON handling.
- `internal/handler/note_test.go`: cover default mode, password mode, encrypted redaction, no `is_locked` in JSON, and strict legacy-field rejection.
- `internal/notes/service.go`: update note views, search, and redaction for MCP.
- `internal/notes/service_test.go`: update service redaction/search tests for `protection_mode`.
- `internal/mcp/server.go`: replace `is_locked` output with `protection_mode`, redact protected body, reject protected note image generation.
- `internal/mcp/auth_test.go` or new `internal/mcp/server_test.go`: cover MCP output shape for protected notes.
- `web/app/src/lib/api/types.ts`: update `Note` type and protection payload types.
- `web/app/src/lib/stores/notes.ts`: allow protection updates and refetch redacted notes when needed.
- `web/app/src/lib/api/notes.ts`: create note-specific helpers for verifying password protection.
- `web/app/src/lib/crypto/noteEncryption.ts`: implement Web Crypto encryption/decryption helpers.
- `web/app/src/lib/crypto/noteEncryption.test.ts`: cover round-trip and wrong-password behavior.
- `web/app/src/lib/stores/dialogs.ts` and `web/app/src/lib/components/common/DialogHost.svelte`: allow password input dialogs.
- `web/app/src/lib/components/common/PasswordField.svelte`: shared password input with show/hide toggle.
- `web/app/src/lib/components/editor/NoteProtectionDialog.svelte`: add protection-mode settings UI.
- `web/app/src/lib/components/editor/EditorPane.svelte`: render protected states, unlock actions, encrypted autosave, and protection settings entry point.
- `web/app/src/lib/stores/preferences.ts`: add localized labels and messages used by the new UI.

This plan does not add folder-level protection UI. If an implementer touches folder settings while executing adjacent work, do not add an `encrypted` option there.

## Task 1: Local Code-Group Source Editing

**Files:**
- Modify: `web/app/src/lib/markdown/codeGroups.ts`
- Modify: `web/app/src/lib/markdown/editorCodeGroups.ts`
- Modify: `web/app/src/lib/components/editor/MarkdownEditor.svelte`
- Test: `web/app/src/lib/markdown/render.test.ts`

**Interfaces:**
- Produces:
  - `interface CodeGroupSourceEditRequest { sourceID: string; startLine: number; endLine: number; raw: string; signature: string; }`
  - `function isEditableCodeGroupSource(markdown: string): boolean`
  - `function replaceCodeGroupSource(markdown: string, request: CodeGroupSourceEditRequest, nextRaw: string): { markdown: string; error?: string }`
  - `type ReplaceCodeGroupSource = (request: CodeGroupSourceEditRequest, nextRaw: string) => string | null`
- Consumes existing:
  - `extractCodeGroupSources(markdown: string)`
  - `renderCodeGroup(items)`

- [ ] **Step 1: Add failing pure helper tests**

Add these tests to `web/app/src/lib/markdown/render.test.ts` inside `describe("renderMarkdown code groups", ...)`:

```ts
import {
  preserveCodeGroups,
  replaceCodeGroupSource,
  isEditableCodeGroupSource,
  extractCodeGroupSources,
} from "./codeGroups";

it("replaces one code-group source block without touching siblings", () => {
  const original = [
    "Intro",
    "::: code-group",
    "```bash [pnpm]",
    "pnpm install",
    "```",
    ":::",
    "Middle",
    "::: code-group",
    "```bash [npm]",
    "npm install",
    "```",
    ":::",
  ].join("\n");
  const first = extractCodeGroupSources(original)[0];
  const nextRaw = [
    "::: code-group",
    "```bash [bun]",
    "bun install",
    "```",
    ":::",
  ].join("\n");

  const result = replaceCodeGroupSource(original, {
    sourceID: "group-1",
    startLine: first.startLine,
    endLine: first.endLine,
    raw: first.raw,
    signature: first.signature,
  }, nextRaw);

  expect(result.error).toBeUndefined();
  expect(result.markdown).toContain("```bash [bun]");
  expect(result.markdown).toContain("```bash [npm]");
  expect(result.markdown).not.toContain("```bash [pnpm]");
});

it("rejects local source replacements that are not code groups", () => {
  expect(isEditableCodeGroupSource("plain text")).toBe(false);

  const original = [
    "::: code-group",
    "```bash [pnpm]",
    "pnpm install",
    "```",
    ":::",
  ].join("\n");
  const group = extractCodeGroupSources(original)[0];
  const result = replaceCodeGroupSource(original, {
    sourceID: "group-1",
    startLine: group.startLine,
    endLine: group.endLine,
    raw: group.raw,
    signature: group.signature,
  }, "plain text");

  expect(result.markdown).toBe(original);
  expect(result.error).toBe("Replacement must be a complete code-group or code-tabs block.");
});
```

- [ ] **Step 2: Run the failing tests**

Run:

```bash
cd web/app
npm test -- src/lib/markdown/render.test.ts
```

Expected: TypeScript/Vitest fails because `replaceCodeGroupSource` and `isEditableCodeGroupSource` are not exported.

- [ ] **Step 3: Add pure helper interfaces and implementation**

Add this to `web/app/src/lib/markdown/codeGroups.ts` near the existing interfaces and helpers:

```ts
export interface CodeGroupSourceEditRequest {
  sourceID: string;
  startLine: number;
  endLine: number;
  raw: string;
  signature: string;
}

export function isEditableCodeGroupSource(markdown: string): boolean {
  const groups = extractCodeGroupSources(markdown.trim());
  return groups.length === 1 && groups[0].raw.trim() === markdown.trim();
}

export function replaceCodeGroupSource(
  markdown: string,
  request: CodeGroupSourceEditRequest,
  nextRaw: string,
): { markdown: string; error?: string } {
  const replacement = nextRaw.trim();
  if (!isEditableCodeGroupSource(replacement)) {
    return {
      markdown,
      error: "Replacement must be a complete code-group or code-tabs block.",
    };
  }

  const lines = markdown.split(/\r?\n/);
  const requested = lines.slice(request.startLine, request.endLine + 1).join("\n");
  if (requested === request.raw) {
    return {
      markdown: [
        ...lines.slice(0, request.startLine),
        ...replacement.split(/\r?\n/),
        ...lines.slice(request.endLine + 1),
      ].join("\n"),
    };
  }

  const currentGroups = extractCodeGroupSources(markdown);
  const matching = currentGroups.find((group) => group.raw === request.raw)
    ?? currentGroups.find((group) => group.signature === request.signature);
  if (!matching) {
    return {
      markdown,
      error: "The original code group could not be found.",
    };
  }

  return {
    markdown: [
      ...lines.slice(0, matching.startLine),
      ...replacement.split(/\r?\n/),
      ...lines.slice(matching.endLine + 1),
    ].join("\n"),
  };
}
```

- [ ] **Step 4: Run pure helper tests until green**

Run:

```bash
cd web/app
npm test -- src/lib/markdown/render.test.ts
```

Expected: PASS for the new code-group replacement tests and existing render tests.

- [ ] **Step 5: Replace the editor plugin callback contract**

Modify imports and types in `web/app/src/lib/markdown/editorCodeGroups.ts`:

```ts
import {
  attachCodeGroupTabs,
  extractCodeGroups,
  renderCodeGroup,
  type CodeGroupBlock,
  type CodeGroupSourceEditRequest,
} from "./codeGroups";

export type ReplaceCodeGroupSource = (
  request: CodeGroupSourceEditRequest,
  nextRaw: string,
) => string | null;
```

Change `createCodeGroupPreview` to accept the full source group and replacement callback:

```ts
function createCodeGroupPreview(
  group: CodeGroupBlock & { raw: string; signature: string },
  sourceID: string,
  replaceSource: ReplaceCodeGroupSource,
): HTMLElement {
  const preview = document.createElement("div");
  preview.className = "editor-code-group-preview";
  preview.contentEditable = "false";

  const sourceButton = document.createElement("button");
  sourceButton.type = "button";
  sourceButton.className = "editor-code-group-source-toggle";
  sourceButton.textContent = "Source";
  sourceButton.title = "Edit this code group source";

  const content = document.createElement("div");
  content.className = "editor-code-group-preview__content";
  content.innerHTML = renderCodeGroup(group.items);

  const editor = document.createElement("div");
  editor.className = "editor-code-group-local-source";
  editor.hidden = true;

  const textarea = document.createElement("textarea");
  textarea.value = group.raw;
  textarea.spellcheck = false;

  const error = document.createElement("p");
  error.className = "editor-code-group-local-source__error";
  error.hidden = true;

  const save = document.createElement("button");
  save.type = "button";
  save.textContent = "Save";

  const cancel = document.createElement("button");
  cancel.type = "button";
  cancel.textContent = "Cancel";

  const actions = document.createElement("div");
  actions.className = "editor-code-group-local-source__actions";
  actions.append(cancel, save);
  editor.append(textarea, error, actions);
  preview.append(sourceButton, content, editor);

  const detach = attachCodeGroupTabs(preview);
  const request: CodeGroupSourceEditRequest = {
    sourceID,
    startLine: group.startLine,
    endLine: group.endLine,
    raw: group.raw,
    signature: group.signature,
  };

  const openEditor = (event: MouseEvent): void => {
    event.preventDefault();
    event.stopPropagation();
    content.hidden = true;
    editor.hidden = false;
    error.hidden = true;
    textarea.focus();
  };
  const closeEditor = (): void => {
    textarea.value = group.raw;
    content.hidden = false;
    editor.hidden = true;
    error.hidden = true;
  };
  const saveSource = (): void => {
    const message = replaceSource(request, textarea.value);
    if (message) {
      error.textContent = message;
      error.hidden = false;
    }
  };

  sourceButton.addEventListener("click", openEditor);
  cancel.addEventListener("click", closeEditor);
  save.addEventListener("click", saveSource);

  previewCleanup.set(preview, () => {
    detach();
    sourceButton.removeEventListener("click", openEditor);
    cancel.removeEventListener("click", closeEditor);
    save.removeEventListener("click", saveSource);
  });
  return preview;
}
```

Update `createCodeGroupEditorPlugin` to accept `replaceSource` instead of `requestSourceMode`.

- [ ] **Step 6: Wire replacement through `MarkdownEditor.svelte`**

Modify `web/app/src/lib/components/editor/MarkdownEditor.svelte`:

```ts
import {
  preserveCodeGroups,
  replaceCodeGroupSource,
  type CodeGroupSourceEditRequest,
} from "../../markdown/codeGroups";
```

Remove the exported `requestSourceMode` prop. Add:

```ts
function replaceLocalCodeGroupSource(
  request: CodeGroupSourceEditRequest,
  nextRaw: string,
): string | null {
  if (!crepe) return "The editor is not ready.";
  const result = replaceCodeGroupSource(lastMarkdown, request, nextRaw);
  if (result.error) return result.error;

  applyingExternalValue = true;
  crepe.editor.action(replaceAll(result.markdown, true));
  lastMarkdown = result.markdown;
  onChange(result.markdown);
  refreshCodeGroupDecorations();
  void tick().then(() => {
    applyingExternalValue = false;
    focusEditor();
  });
  return null;
}
```

Change plugin registration:

```ts
crepe.editor.use(createCodeGroupEditorPlugin(() => lastMarkdown, replaceLocalCodeGroupSource));
```

Remove `requestSourceMode` from `EditorPane.svelte`'s `MarkdownEditorComponent` type and `<svelte:component ... />` props.

- [ ] **Step 7: Add CSS for the local source editor**

Modify `web/app/src/lib/styles/global.css` near existing `.editor-code-group-*` rules:

```css
.markdown-editor-host .editor-code-group-local-source {
  border: 1px solid var(--border-subtle);
  background: var(--surface-muted);
  padding: 0.75rem;
}

.markdown-editor-host .editor-code-group-local-source textarea {
  width: 100%;
  min-height: 10rem;
  resize: vertical;
  font: 13px/1.5 var(--font-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  color: var(--text-primary);
  background: var(--surface);
  border: 1px solid var(--border-subtle);
}

.markdown-editor-host .editor-code-group-local-source__actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  margin-top: 0.5rem;
}

.markdown-editor-host .editor-code-group-local-source__error {
  margin: 0.5rem 0 0;
  color: var(--danger-text, #b42318);
  font-size: 0.8125rem;
}
```

Use these exact fallback-backed CSS variables so the block compiles even if a theme token is absent: `var(--surface-muted, #f6f7f8)`, `var(--surface, #ffffff)`, `var(--border-subtle, #d8dde3)`, `var(--text-primary, #1f2933)`, and `var(--danger-text, #b42318)`.

- [ ] **Step 8: Run frontend checks**

Run:

```bash
cd web/app
npm test -- src/lib/markdown/render.test.ts
npm run check
```

Expected: both commands pass.

- [ ] **Step 9: Commit**

```bash
git add web/app/src/lib/markdown/codeGroups.ts web/app/src/lib/markdown/editorCodeGroups.ts web/app/src/lib/components/editor/MarkdownEditor.svelte web/app/src/lib/components/editor/EditorPane.svelte web/app/src/lib/markdown/render.test.ts web/app/src/lib/styles/global.css
git commit -m "Make code group source editing local"
```

## Task 2: Backend Schema and Destructive Legacy Field Removal

**Files:**
- Modify: `ent/schema/note.go`
- Modify: `cmd/server/main.go`
- Generated: `ent/*`
- Test: `internal/handler/note_test.go`

**Interfaces:**
- Produces Ent note fields:
  - `protection_mode`
  - `protection_password_hash`
  - `encrypted_content`
  - `encryption_alg`
  - `encryption_kdf`
  - `encryption_salt`
  - `encryption_nonce`
- Removes Ent note fields:
  - `password`
  - `is_locked`

- [ ] **Step 1: Write failing schema behavior test**

Add to `internal/handler/note_test.go`:

```go
func TestNoteProtectionModeDefaultsToNone(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestNoteProtectionModeDefaultsToNone?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	n := client.Note.Create().
		SetTitle("Plain note").
		SetContent("plain").
		SetUserID(u.ID).
		SaveX(ctx)

	if string(n.ProtectionMode) != "none" {
		t.Fatalf("expected protection_mode none, got %q", n.ProtectionMode)
	}
}
```

- [ ] **Step 2: Run failing Go test**

Run:

```bash
go test ./internal/handler -run TestNoteProtectionModeDefaultsToNone -count=1
```

Expected: compile fails because `ProtectionMode` is not generated.

- [ ] **Step 3: Update note schema**

Modify `ent/schema/note.go` fields:

```go
field.Text("content").
	Optional(),
field.String("color").
	Optional().
	Default(""),
field.Enum("protection_mode").
	Values("none", "password", "encrypted").
	Default("none"),
field.String("protection_password_hash").
	Optional().
	Sensitive(),
field.Text("encrypted_content").
	Optional().
	Sensitive(),
field.String("encryption_alg").
	Optional(),
field.String("encryption_kdf").
	Optional(),
field.String("encryption_salt").
	Optional(),
field.String("encryption_nonce").
	Optional(),
field.Bool("is_starred").
	Default(false),
```

Delete these old field definitions from the schema:

```go
field.String("password").
	Optional().
	Sensitive(),
field.Bool("is_locked").
	Default(false),
```

- [ ] **Step 4: Enable drop-column migration**

Modify `cmd/server/main.go` imports:

```go
import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"smarticky/ent"
	"smarticky/ent/migrate"
	...
)
```

Modify startup migration:

```go
if err := client.Schema.Create(context.Background(), migrate.WithDropColumn(true)); err != nil {
	zap.L().Warn("Schema migration failed, trying to continue", zap.Error(err))
}
```

- [ ] **Step 5: Regenerate Ent**

Run:

```bash
go generate ./ent
```

Expected: generated note files include `ProtectionMode` fields and no generated `FieldIsLocked` / `FieldPassword` constants.

- [ ] **Step 6: Run schema test**

Run:

```bash
go test ./internal/handler -run TestNoteProtectionModeDefaultsToNone -count=1
```

Expected: PASS.

- [ ] **Step 7: Run a no-legacy-symbol check**

Run:

```bash
rg -n "FieldIsLocked|FieldPassword|SetIsLocked|SetPassword|IsLocked|\\.Password" ent internal web/app/src
```

Expected: matches remain only in tests or files that have not been migrated yet. Record the output for Task 3.

- [ ] **Step 8: Commit**

```bash
git add ent cmd/server/main.go internal/handler/note_test.go
git commit -m "Replace legacy note lock schema"
```

## Task 3: Backend Protection API, Redaction, Search, and MCP

**Files:**
- Modify: `internal/handler/note.go`
- Modify: `internal/handler/note_test.go`
- Modify: `internal/notes/service.go`
- Modify: `internal/notes/service_test.go`
- Modify: `internal/mcp/server.go`
- Test: `internal/mcp/auth_test.go` or new `internal/mcp/server_test.go`

**Interfaces:**
- Produces JSON fields:
  - `protection_mode`
  - `content_redacted`
  - `encrypted_content`
  - `encryption_alg`
  - `encryption_kdf`
  - `encryption_salt`
  - `encryption_nonce`
- Consumes update JSON fields:
  - `protection_mode`
  - `protection_password`
  - `encrypted_content`
  - `encryption_alg`
  - `encryption_kdf`
  - `encryption_salt`
  - `encryption_nonce`

- [ ] **Step 1: Add handler tests for response shape and strict legacy rejection**

Add tests to `internal/handler/note_test.go`:

```go
func TestUpdateNoteRejectsLegacyIsLockedField(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestUpdateNoteRejectsLegacyIsLockedField?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()
	u := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)
	n := client.Note.Create().SetTitle("Note").SetUserID(u.ID).SaveX(ctx)

	req := httptest.NewRequest(http.MethodPut, "/api/notes/"+n.ID.String(), strings.NewReader(`{"is_locked":true}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("user_id", u.ID)
	c.SetParamNames("id")
	c.SetParamValues(n.ID.String())

	if err := NewHandler(client, nil).UpdateNote(c); err != nil {
		t.Fatalf("UpdateNote returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPasswordProtectedNoteRedactsUntilVerified(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestPasswordProtectedNoteRedactsUntilVerified?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()
	u := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)
	hash, err := hashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	n := client.Note.Create().
		SetTitle("Protected").
		SetContent("private body").
		SetProtectionMode(note.ProtectionModePassword).
		SetProtectionPasswordHash(hash).
		SetUserID(u.ID).
		SaveX(ctx)

	h := NewHandler(client, nil)
	e := echo.New()
	getReq := httptest.NewRequest(http.MethodGet, "/api/notes/"+n.ID.String(), nil)
	getRec := httptest.NewRecorder()
	getCtx := e.NewContext(getReq, getRec)
	getCtx.Set("user_id", u.ID)
	getCtx.SetParamNames("id")
	getCtx.SetParamValues(n.ID.String())
	if err := h.GetNote(getCtx); err != nil {
		t.Fatalf("GetNote returned error: %v", err)
	}
	var redacted NoteResponse
	if err := json.NewDecoder(getRec.Body).Decode(&redacted); err != nil {
		t.Fatalf("decode redacted response: %v", err)
	}
	if redacted.Content != "" || !redacted.ContentRedacted || redacted.ProtectionMode != "password" {
		t.Fatalf("expected redacted password note, got %+v", redacted)
	}

	verifyReq := httptest.NewRequest(http.MethodPost, "/api/notes/"+n.ID.String()+"/verify-password", strings.NewReader(`{"password":"secret"}`))
	verifyReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	verifyRec := httptest.NewRecorder()
	verifyCtx := e.NewContext(verifyReq, verifyRec)
	verifyCtx.Set("user_id", u.ID)
	verifyCtx.SetParamNames("id")
	verifyCtx.SetParamValues(n.ID.String())
	if err := h.VerifyNotePassword(verifyCtx); err != nil {
		t.Fatalf("VerifyNotePassword returned error: %v", err)
	}
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", verifyRec.Code, verifyRec.Body.String())
	}
	if !strings.Contains(verifyRec.Body.String(), "private body") {
		t.Fatalf("expected verified response to include content, got %s", verifyRec.Body.String())
	}
}
```

- [ ] **Step 2: Run failing handler tests**

Run:

```bash
go test ./internal/handler -run 'Test(UpdateNoteRejectsLegacyIsLockedField|PasswordProtectedNoteRedactsUntilVerified)' -count=1
```

Expected: compile or test failures because response/request fields are not implemented.

- [ ] **Step 3: Update handler request/response structs**

Modify `internal/handler/note.go`:

```go
type NoteResponse struct {
	ID                  uuid.UUID  `json:"id"`
	Title               string     `json:"title"`
	Content             string     `json:"content"`
	Color               string     `json:"color"`
	ProtectionMode      string     `json:"protection_mode"`
	ContentRedacted     bool       `json:"content_redacted"`
	EncryptedContent    string     `json:"encrypted_content,omitempty"`
	EncryptionAlg       string     `json:"encryption_alg,omitempty"`
	EncryptionKDF       string     `json:"encryption_kdf,omitempty"`
	EncryptionSalt      string     `json:"encryption_salt,omitempty"`
	EncryptionNonce     string     `json:"encryption_nonce,omitempty"`
	IsStarred           bool       `json:"is_starred"`
	IsDeleted           bool       `json:"is_deleted"`
	FolderID            *uuid.UUID `json:"folder_id"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type UpdateNoteRequest struct {
	Title                  *string      `json:"title"`
	Content                *string      `json:"content"`
	Color                  *string      `json:"color"`
	ProtectionMode         *string      `json:"protection_mode"`
	ProtectionPassword     *string      `json:"protection_password"`
	EncryptedContent       *string      `json:"encrypted_content"`
	EncryptionAlg          *string      `json:"encryption_alg"`
	EncryptionKDF          *string      `json:"encryption_kdf"`
	EncryptionSalt         *string      `json:"encryption_salt"`
	EncryptionNonce        *string      `json:"encryption_nonce"`
	IsStarred              *bool        `json:"is_starred"`
	IsDeleted              *bool        `json:"is_deleted"`
	FolderID               OptionalUUID `json:"folder_id"`
}
```

Add strict JSON binding helper in `internal/handler/utils.go`:

```go
func bindStrictJSON(c echo.Context, dst any) error {
	decoder := json.NewDecoder(c.Request().Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	return nil
}
```

Import `encoding/json` in `utils.go`. Use `bindStrictJSON` in `UpdateNote`.

- [ ] **Step 4: Implement redaction in `noteToResponse`**

Change signature:

```go
func noteToResponse(ctx context.Context, n *ent.Note, revealContent bool) (NoteResponse, error)
```

Use this body logic:

```go
content := n.Content
redacted := false
if !revealContent {
	switch n.ProtectionMode {
	case note.ProtectionModePassword:
		content = ""
		redacted = true
	case note.ProtectionModeEncrypted:
		content = ""
		redacted = true
	}
}

return NoteResponse{
	ID:               n.ID,
	Title:            n.Title,
	Content:          content,
	Color:            n.Color,
	ProtectionMode:   string(n.ProtectionMode),
	ContentRedacted:  redacted,
	EncryptedContent: n.EncryptedContent,
	EncryptionAlg:    n.EncryptionAlg,
	EncryptionKDF:    n.EncryptionKDF,
	EncryptionSalt:   n.EncryptionSalt,
	EncryptionNonce:  n.EncryptionNonce,
	IsStarred:        n.IsStarred,
	IsDeleted:        n.IsDeleted,
	FolderID:         folderID,
	CreatedAt:        n.CreatedAt,
	UpdatedAt:        n.UpdatedAt,
}, nil
```

Call `noteToResponse(ctx, n, false)` from list/get/create/update, and `noteToResponse(ctx, n, true)` only from successful password verification.

- [ ] **Step 5: Implement protection updates**

In `UpdateNote`, after title/content/color handling:

```go
if req.ProtectionMode != nil {
	switch *req.ProtectionMode {
	case "none":
		update.SetProtectionMode(note.ProtectionModeNone)
		update.ClearProtectionPasswordHash()
		update.ClearEncryptedContent()
		update.ClearEncryptionAlg()
		update.ClearEncryptionKDF()
		update.ClearEncryptionSalt()
		update.ClearEncryptionNonce()
	case "password":
		if req.ProtectionPassword == nil || strings.TrimSpace(*req.ProtectionPassword) == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "protection_password is required"})
		}
		hashedPassword, err := hashPassword(*req.ProtectionPassword)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
		}
		update.SetProtectionMode(note.ProtectionModePassword)
		update.SetProtectionPasswordHash(hashedPassword)
		update.ClearEncryptedContent()
		update.ClearEncryptionAlg()
		update.ClearEncryptionKDF()
		update.ClearEncryptionSalt()
		update.ClearEncryptionNonce()
	case "encrypted":
		if req.EncryptedContent == nil || *req.EncryptedContent == "" ||
			req.EncryptionAlg == nil || *req.EncryptionAlg == "" ||
			req.EncryptionKDF == nil || *req.EncryptionKDF == "" ||
			req.EncryptionSalt == nil || *req.EncryptionSalt == "" ||
			req.EncryptionNonce == nil || *req.EncryptionNonce == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "encrypted payload is required"})
		}
		update.SetProtectionMode(note.ProtectionModeEncrypted)
		update.SetContent("")
		update.ClearProtectionPasswordHash()
		update.SetEncryptedContent(*req.EncryptedContent)
		update.SetEncryptionAlg(*req.EncryptionAlg)
		update.SetEncryptionKDF(*req.EncryptionKDF)
		update.SetEncryptionSalt(*req.EncryptionSalt)
		update.SetEncryptionNonce(*req.EncryptionNonce)
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid protection_mode"})
	}
}
```

If `req.Content != nil` and the existing or requested mode is encrypted, only accept it when switching to `"none"` or `"password"` in the same request.

- [ ] **Step 6: Update password verification endpoint**

Change verification checks:

```go
if n.ProtectionMode != note.ProtectionModePassword || n.ProtectionPasswordHash == "" {
	return c.JSON(http.StatusBadRequest, map[string]string{"error": "note is not password protected"})
}

valid, err := verifyPassword(req.Password, n.ProtectionPasswordHash)
```

Return:

```go
response, err := noteToResponse(ctx, n, true)
```

- [ ] **Step 7: Update notes service and MCP structs**

In `internal/notes/service.go`, replace `IsLocked` with:

```go
ProtectionMode  string `json:"protection_mode"`
ContentRedacted bool   `json:"content_redacted"`
```

In search query, do not body-search encrypted notes:

```go
query.Where(note.Or(
	note.TitleContainsFold(q),
	note.And(
		note.ProtectionModeNEQ(note.ProtectionModeEncrypted),
		note.ContentContainsFold(q),
	),
))
```

In `noteToView`, redact when `redactLocked` is true and `row.ProtectionMode != note.ProtectionModeNone`.

In `internal/mcp/server.go`, change `mcpNote`:

```go
ProtectionMode  string `json:"protection_mode"`
ContentRedacted bool   `json:"content_redacted"`
```

Remove `IsLocked`. Reject note image generation when:

```go
if row.ProtectionMode != "none" {
	return nil, imageOutput{}, errors.New("protected notes cannot be rendered through MCP")
}
```

- [ ] **Step 8: Run backend tests**

Run:

```bash
go test ./internal/handler ./internal/notes ./internal/mcp -count=1
```

Expected: PASS.

- [ ] **Step 9: Run no-legacy-output check**

Run:

```bash
rg -n "is_locked|IsLocked|FieldIsLocked|SetIsLocked|FieldPassword|SetPassword|\\.Password" internal ent web/app/src docs/superpowers/plans/2026-06-22-note-protection-code-group.md
```

Expected: matches are limited to the design/plan references to removed legacy fields and no generated Ent field or API output structs contain `is_locked`.

- [ ] **Step 10: Commit**

```bash
git add internal ent cmd/server/main.go
git commit -m "Add note protection backend"
```

## Task 4: Frontend Types, Store API, and Password Dialog Support

**Files:**
- Modify: `web/app/src/lib/api/types.ts`
- Create: `web/app/src/lib/api/notes.ts`
- Create: `web/app/src/lib/components/common/PasswordField.svelte`
- Modify: `web/app/src/lib/stores/notes.ts`
- Modify: `web/app/src/lib/stores/dialogs.ts`
- Modify: `web/app/src/lib/components/common/DialogHost.svelte`
- Modify: `web/app/src/lib/stores/preferences.ts`

**Interfaces:**
- Produces:
  - `type NoteProtectionMode = "none" | "password" | "encrypted"`
  - `verifyNotePassword(noteID: string, password: string): Promise<Note>`
  - `PasswordField.svelte` with `bind:value`, label, autocomplete, invalid/describedBy props, and internal show/hide toggle.
  - `inputDialog({ inputType: "text" | "password", ... })`

- [ ] **Step 1: Update API types**

Modify `web/app/src/lib/api/types.ts`:

```ts
export type NoteProtectionMode = "none" | "password" | "encrypted";

export interface Note {
  id: UUID;
  title: string;
  content: string;
  color: string;
  protection_mode: NoteProtectionMode;
  content_redacted?: boolean;
  encrypted_content?: string;
  encryption_alg?: string;
  encryption_kdf?: string;
  encryption_salt?: string;
  encryption_nonce?: string;
  is_starred: boolean;
  is_deleted: boolean;
  folder_id?: UUID | null;
  tags?: Tag[];
  created_at: string;
  updated_at: string;
}
```

- [ ] **Step 2: Add note API helper**

Create `web/app/src/lib/api/notes.ts`:

```ts
import { apiFetch } from "./client";
import type { Note } from "./types";

export async function verifyNotePassword(
  noteID: string,
  password: string,
): Promise<Note> {
  const response = await apiFetch<{ success: boolean; note: Note }>(
    `/notes/${noteID}/verify-password`,
    {
      method: "POST",
      body: JSON.stringify({ password }),
    },
  );
  return response.note;
}
```

- [ ] **Step 3: Update note store update fields**

Modify `web/app/src/lib/stores/notes.ts`:

```ts
type NoteUpdateFields = Partial<
  Pick<
    Note,
    | "title"
    | "content"
    | "color"
    | "is_starred"
    | "is_deleted"
    | "folder_id"
    | "protection_mode"
    | "encrypted_content"
    | "encryption_alg"
    | "encryption_kdf"
    | "encryption_salt"
    | "encryption_nonce"
  >
> & {
  protection_password?: string;
};
```

Update `getByID` so redacted selected/listed notes are refetched:

```ts
if (state.selected?.id === noteId && !state.selected.content_redacted) return state.selected;
const listed = state.notes.find((note) => note.id === noteId);
if (listed && !listed.content_redacted) return listed;
return apiFetch<Note>(`/notes/${noteId}`);
```

- [ ] **Step 4: Add shared password field component**

Create `web/app/src/lib/components/common/PasswordField.svelte`:

```svelte
<script lang="ts">
  import { Eye, EyeOff } from "@lucide/svelte";
  import { preferencesStore, t } from "../../stores/preferences";

  export let value = "";
  export let label = "";
  export let autocomplete = "current-password";
  export let placeholder: string | undefined = undefined;
  export let invalid = false;
  export let describedBy: string | undefined = undefined;
  export let disabled = false;

  let visible = false;
</script>

<label class="password-field">
  <span>{label}</span>
  <span class="password-field__control">
    <input
      bind:value
      type={visible ? "text" : "password"}
      {autocomplete}
      {placeholder}
      aria-invalid={invalid ? "true" : "false"}
      aria-describedby={describedBy}
      {disabled}
    />
    <button
      type="button"
      aria-label={visible
        ? t("hidePassword", $preferencesStore.language)
        : t("showPassword", $preferencesStore.language)}
      title={visible
        ? t("hidePassword", $preferencesStore.language)
        : t("showPassword", $preferencesStore.language)}
      on:click={() => (visible = !visible)}
      disabled={disabled}
    >
      {#if visible}
        <EyeOff size={16} strokeWidth={2} aria-hidden="true" />
      {:else}
        <Eye size={16} strokeWidth={2} aria-hidden="true" />
      {/if}
    </button>
  </span>
</label>
```

Add CSS to `web/app/src/lib/styles/global.css`:

```css
.password-field {
  display: grid;
  gap: 0.4rem;
}

.password-field__control {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  border: 1px solid var(--color-border, rgba(0, 0, 0, 0.14));
  background: var(--color-surface, #fff);
}

.password-field__control input {
  min-width: 0;
  border: 0;
  background: transparent;
}

.password-field__control button {
  inline-size: 2.25rem;
  block-size: 2.25rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  background: transparent;
  color: var(--color-text-muted, #5f6772);
}
```

- [ ] **Step 5: Support password input dialogs**

Modify `web/app/src/lib/stores/dialogs.ts`:

```ts
interface InputRequest {
  title: string;
  label: string;
  message?: string;
  initialValue?: string;
  placeholder?: string;
  inputType?: "text" | "password";
  confirmLabel: string;
  cancelLabel: string;
  requiredMessage: string;
  resolve: (value: string | null) => void;
}
```

Modify `web/app/src/lib/components/common/DialogHost.svelte` input:

```svelte
<script lang="ts">
  import PasswordField from "./PasswordField.svelte";
  ...
</script>
```

Render `PasswordField` when `$inputRequest.inputType === "password"`:

```svelte
{#if $inputRequest.inputType === "password"}
  <PasswordField
    bind:value={inputValue}
    label={$inputRequest.label}
    autocomplete="current-password"
    placeholder={$inputRequest.placeholder}
    invalid={Boolean(inputError)}
    describedBy={inputError ? "input-dialog-error" : undefined}
  />
{:else}
  <label class="input-dialog__field">
    <span>{$inputRequest.label}</span>
    <input
      bind:this={inputElement}
      bind:value={inputValue}
      type="text"
      autocomplete="off"
      placeholder={$inputRequest.placeholder}
      aria-invalid={inputError ? "true" : "false"}
      aria-describedby={inputError ? "input-dialog-error" : undefined}
    />
  </label>
{/if}
```

Remove the old single unconditional input block:

```svelte
<input
  bind:this={inputElement}
  bind:value={inputValue}
  type={$inputRequest.inputType ?? "text"}
  autocomplete={$inputRequest.inputType === "password" ? "current-password" : "off"}
  placeholder={$inputRequest.placeholder}
  aria-invalid={inputError ? "true" : "false"}
  aria-describedby={inputError ? "input-dialog-error" : undefined}
/>
```

- [ ] **Step 6: Add preference strings**

Add Chinese and English keys in `web/app/src/lib/stores/preferences.ts`:

```ts
protectNote: "保护笔记",
removeProtection: "取消保护",
accessPassword: "访问密码",
encryptContent: "加密正文",
unlockNote: "解锁笔记",
noteProtected: "笔记已保护",
noteUnlocked: "笔记已解锁",
noteUnlockFailed: "解锁失败",
encryptionPasswordWarning: "加密密码无法找回，请妥善保存。",
showPassword: "显示密码",
hidePassword: "隐藏密码",
```

English:

```ts
protectNote: "Protect note",
removeProtection: "Remove protection",
accessPassword: "Access password",
encryptContent: "Encrypt content",
unlockNote: "Unlock note",
noteProtected: "Note protected",
noteUnlocked: "Note unlocked",
noteUnlockFailed: "Unlock failed",
encryptionPasswordWarning: "Encryption passwords cannot be recovered.",
showPassword: "Show password",
hidePassword: "Hide password",
```

- [ ] **Step 7: Run frontend check**

Run:

```bash
cd web/app
npm run check
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add web/app/src/lib/api/types.ts web/app/src/lib/api/notes.ts web/app/src/lib/components/common/PasswordField.svelte web/app/src/lib/stores/notes.ts web/app/src/lib/stores/dialogs.ts web/app/src/lib/components/common/DialogHost.svelte web/app/src/lib/stores/preferences.ts web/app/src/lib/styles/global.css
git commit -m "Add note protection frontend types"
```

## Task 5: Client-Side Zero-Knowledge Encryption Helpers

**Files:**
- Create: `web/app/src/lib/crypto/noteEncryption.ts`
- Create: `web/app/src/lib/crypto/noteEncryption.test.ts`

**Interfaces:**
- Produces:
  - `encryptNoteContent(content: string, password: string): Promise<EncryptedNotePayload>`
  - `decryptNoteContent(payload: EncryptedNotePayload, password: string): Promise<string>`
  - `interface EncryptedNotePayload`

- [ ] **Step 1: Add failing crypto tests**

Create `web/app/src/lib/crypto/noteEncryption.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { decryptNoteContent, encryptNoteContent } from "./noteEncryption";

describe("note encryption", () => {
  it("round-trips note content without exposing plaintext in ciphertext", async () => {
    const payload = await encryptNoteContent("private body", "correct horse");

    expect(payload.encrypted_content).not.toContain("private body");
    expect(payload.encryption_alg).toBe("AES-GCM");
    expect(payload.encryption_kdf).toMatch(/^PBKDF2-SHA-256:/);

    await expect(decryptNoteContent(payload, "correct horse")).resolves.toBe("private body");
  });

  it("rejects wrong passwords", async () => {
    const payload = await encryptNoteContent("private body", "correct horse");

    await expect(decryptNoteContent(payload, "wrong password")).rejects.toThrow(
      "Unable to decrypt note content.",
    );
  });
});
```

- [ ] **Step 2: Run failing crypto tests**

Run:

```bash
cd web/app
npm test -- src/lib/crypto/noteEncryption.test.ts
```

Expected: fails because `noteEncryption.ts` does not exist.

- [ ] **Step 3: Implement Web Crypto helpers**

Create `web/app/src/lib/crypto/noteEncryption.ts`:

```ts
const textEncoder = new TextEncoder();
const textDecoder = new TextDecoder();
const iterations = 310_000;

export interface EncryptedNotePayload {
  encrypted_content: string;
  encryption_alg: "AES-GCM";
  encryption_kdf: string;
  encryption_salt: string;
  encryption_nonce: string;
}

function bytesToBase64(bytes: Uint8Array): string {
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}

function base64ToBytes(value: string): Uint8Array {
  const binary = atob(value);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index);
  }
  return bytes;
}

async function deriveKey(password: string, salt: Uint8Array): Promise<CryptoKey> {
  const baseKey = await crypto.subtle.importKey(
    "raw",
    textEncoder.encode(password),
    "PBKDF2",
    false,
    ["deriveKey"],
  );
  return crypto.subtle.deriveKey(
    {
      name: "PBKDF2",
      hash: "SHA-256",
      salt,
      iterations,
    },
    baseKey,
    { name: "AES-GCM", length: 256 },
    false,
    ["encrypt", "decrypt"],
  );
}

export async function encryptNoteContent(
  content: string,
  password: string,
): Promise<EncryptedNotePayload> {
  const salt = crypto.getRandomValues(new Uint8Array(16));
  const nonce = crypto.getRandomValues(new Uint8Array(12));
  const key = await deriveKey(password, salt);
  const encrypted = await crypto.subtle.encrypt(
    { name: "AES-GCM", iv: nonce },
    key,
    textEncoder.encode(content),
  );

  return {
    encrypted_content: bytesToBase64(new Uint8Array(encrypted)),
    encryption_alg: "AES-GCM",
    encryption_kdf: `PBKDF2-SHA-256:${iterations}`,
    encryption_salt: bytesToBase64(salt),
    encryption_nonce: bytesToBase64(nonce),
  };
}

export async function decryptNoteContent(
  payload: EncryptedNotePayload,
  password: string,
): Promise<string> {
  try {
    const salt = base64ToBytes(payload.encryption_salt);
    const nonce = base64ToBytes(payload.encryption_nonce);
    const key = await deriveKey(password, salt);
    const decrypted = await crypto.subtle.decrypt(
      { name: "AES-GCM", iv: nonce },
      key,
      base64ToBytes(payload.encrypted_content),
    );
    return textDecoder.decode(decrypted);
  } catch {
    throw new Error("Unable to decrypt note content.");
  }
}
```

- [ ] **Step 4: Run crypto tests**

Run:

```bash
cd web/app
npm test -- src/lib/crypto/noteEncryption.test.ts
```

Expected: PASS.

- [ ] **Step 5: Run frontend check**

Run:

```bash
cd web/app
npm run check
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/app/src/lib/crypto/noteEncryption.ts web/app/src/lib/crypto/noteEncryption.test.ts
git commit -m "Add client note encryption helpers"
```

## Task 6: Editor Protection UI and Protected Draft Flow

**Files:**
- Create: `web/app/src/lib/components/editor/NoteProtectionDialog.svelte`
- Modify: `web/app/src/lib/components/common/PasswordField.svelte` only if Task 4 left an integration defect
- Modify: `web/app/src/lib/components/editor/EditorPane.svelte`
- Modify: `web/app/src/lib/styles/global.css`
- Modify: `web/app/src/lib/stores/preferences.ts`

**Interfaces:**
- Consumes:
  - `verifyNotePassword(noteID, password)`
  - `encryptNoteContent(content, password)`
  - `decryptNoteContent(payload, password)`
  - `notesStore.updateSelected(fields)`
- Produces:
  - Editor protected state for password/encrypted notes.
  - Protection settings dialog.

- [ ] **Step 1: Create protection dialog component**

Create `web/app/src/lib/components/editor/NoteProtectionDialog.svelte`:

```svelte
<script lang="ts">
  import type { NoteProtectionMode } from "../../api/types";
  import PasswordField from "../common/PasswordField.svelte";
  import { preferencesStore, t } from "../../stores/preferences";

  export let currentMode: NoteProtectionMode = "none";
  export let onSave: (mode: NoteProtectionMode, password: string) => void = () => {};
  export let onClose: () => void = () => {};

  let mode: NoteProtectionMode = currentMode;
  let password = "";
  let confirmPassword = "";
  let error = "";

  function submit(): void {
    if (mode !== "none") {
      if (!password || password !== confirmPassword) {
        error = t("passwordNotMatch", $preferencesStore.language);
        return;
      }
      if (password.length < 6) {
        error = t("passwordTooShort", $preferencesStore.language);
        return;
      }
    }
    onSave(mode, password);
  }
</script>

<div class="dialog-backdrop" role="presentation">
  <div class="confirm-dialog note-protection-dialog" role="dialog" aria-modal="true">
    <h2>{t("protectNote", $preferencesStore.language)}</h2>
    <div class="note-protection-dialog__modes">
      <label><input bind:group={mode} type="radio" value="none" /> {t("removeProtection", $preferencesStore.language)}</label>
      <label><input bind:group={mode} type="radio" value="password" /> {t("accessPassword", $preferencesStore.language)}</label>
      <label><input bind:group={mode} type="radio" value="encrypted" /> {t("encryptContent", $preferencesStore.language)}</label>
    </div>
    {#if mode === "encrypted"}
      <p>{t("encryptionPasswordWarning", $preferencesStore.language)}</p>
    {/if}
    {#if mode !== "none"}
      <PasswordField
        bind:value={password}
        label={t("password", $preferencesStore.language)}
        autocomplete="new-password"
        invalid={Boolean(error)}
        describedBy={error ? "note-protection-error" : undefined}
      />
      <PasswordField
        bind:value={confirmPassword}
        label={t("confirmPassword", $preferencesStore.language)}
        autocomplete="new-password"
        invalid={Boolean(error)}
        describedBy={error ? "note-protection-error" : undefined}
      />
    {/if}
    {#if error}
      <p class="input-dialog__error" id="note-protection-error">{error}</p>
    {/if}
    <div class="confirm-dialog__actions">
      <button type="button" on:click={onClose}>{t("cancel", $preferencesStore.language)}</button>
      <button class="primary" type="button" on:click={submit}>{t("save", $preferencesStore.language)}</button>
    </div>
  </div>
</div>
```

- [ ] **Step 2: Add protected editor state to `EditorPane.svelte`**

Add imports:

```ts
import NoteProtectionDialog from "./NoteProtectionDialog.svelte";
import { verifyNotePassword } from "../../api/notes";
import { decryptNoteContent, encryptNoteContent } from "../../crypto/noteEncryption";
```

Add state:

```ts
let protectionOpen = false;
let encryptedPassword = "";
let protectedUnlockBusy = false;
let protectedUnlockError = "";

$: noteProtected = note?.protection_mode === "password" || note?.protection_mode === "encrypted";
$: noteLocked = Boolean(noteProtected && note?.content_redacted && !draftContent);
```

In `resetDraft`, set:

```ts
draftContent = nextNote?.content_redacted ? "" : (nextNote?.content ?? "");
encryptedPassword = "";
protectedUnlockError = "";
protectionOpen = false;
```

- [ ] **Step 3: Add unlock actions**

Add functions to `EditorPane.svelte`:

```ts
async function unlockPasswordNote(): Promise<void> {
  if (!note) return;
  const password = await inputDialog({
    title: t("unlockNote", $preferencesStore.language),
    label: t("password", $preferencesStore.language),
    inputType: "password",
    confirmLabel: t("unlockNote", $preferencesStore.language),
    cancelLabel: t("cancel", $preferencesStore.language),
    requiredMessage: t("passwordRequired", $preferencesStore.language),
  });
  if (!password) return;
  protectedUnlockBusy = true;
  try {
    const unlocked = await verifyNotePassword(note.id, password);
    notesStore.replaceSelected(unlocked);
    draftContent = unlocked.content;
    notify(t("noteUnlocked", $preferencesStore.language), "success");
  } catch {
    notify(t("noteUnlockFailed", $preferencesStore.language), "error");
  } finally {
    protectedUnlockBusy = false;
  }
}

async function unlockEncryptedNote(): Promise<void> {
  if (!note?.encrypted_content || !note.encryption_alg || !note.encryption_kdf || !note.encryption_salt || !note.encryption_nonce) return;
  const password = await inputDialog({
    title: t("unlockNote", $preferencesStore.language),
    label: t("password", $preferencesStore.language),
    inputType: "password",
    confirmLabel: t("unlockNote", $preferencesStore.language),
    cancelLabel: t("cancel", $preferencesStore.language),
    requiredMessage: t("passwordRequired", $preferencesStore.language),
  });
  if (!password) return;
  protectedUnlockBusy = true;
  try {
    draftContent = await decryptNoteContent({
      encrypted_content: note.encrypted_content,
      encryption_alg: "AES-GCM",
      encryption_kdf: note.encryption_kdf,
      encryption_salt: note.encryption_salt,
      encryption_nonce: note.encryption_nonce,
    }, password);
    encryptedPassword = password;
    notify(t("noteUnlocked", $preferencesStore.language), "success");
  } catch {
    notify(t("noteUnlockFailed", $preferencesStore.language), "error");
  } finally {
    protectedUnlockBusy = false;
  }
}
```

- [ ] **Step 4: Encrypt autosaves for encrypted notes**

Modify `persistDraft` content handling:

```ts
const outgoing = { ...fields };
if (note?.protection_mode === "encrypted" && "content" in outgoing) {
  if (!encryptedPassword) return;
  const payload = await encryptNoteContent(String(outgoing.content ?? ""), encryptedPassword);
  delete outgoing.content;
  Object.assign(outgoing, {
    protection_mode: "encrypted",
    content: "",
    ...payload,
  });
}
await notesStore.updateSelected(outgoing);
```

Keep local `draftContent` as plaintext while the note stays open.

- [ ] **Step 5: Implement protection settings save**

Add function:

```ts
async function saveProtection(mode: NoteProtectionMode, password: string): Promise<void> {
  if (!note) return;
  try {
    if (mode === "none") {
      await notesStore.updateSelected({
        protection_mode: "none",
        content: draftContent,
        encrypted_content: "",
        encryption_alg: "",
        encryption_kdf: "",
        encryption_salt: "",
        encryption_nonce: "",
      });
      encryptedPassword = "";
    } else if (mode === "password") {
      await notesStore.updateSelected({
        protection_mode: "password",
        protection_password: password,
        content: draftContent,
      });
      encryptedPassword = "";
    } else {
      const payload = await encryptNoteContent(draftContent, password);
      await notesStore.updateSelected({
        protection_mode: "encrypted",
        content: "",
        ...payload,
      });
      encryptedPassword = password;
    }
    protectionOpen = false;
    notify(t("noteProtected", $preferencesStore.language), "success");
  } catch {
    notify(t("saveError", $preferencesStore.language), "error");
  }
}
```

- [ ] **Step 6: Render protected state and settings entry**

Add action menu item:

```svelte
<button
  class="editor-action-button"
  type="button"
  on:click={() => void runMenuAction(() => { protectionOpen = true; })}
>
  {t("protectNote", $preferencesStore.language)}
</button>
```

Inside editor surface before title/content editor:

```svelte
{#if noteLocked}
  <div class="editor-protected-state">
    <p>{note.protection_mode === "encrypted" ? t("encryptContent", $preferencesStore.language) : t("accessPassword", $preferencesStore.language)}</p>
    <button
      class="editor-action-button"
      type="button"
      disabled={protectedUnlockBusy}
      on:click={() => note.protection_mode === "encrypted" ? void unlockEncryptedNote() : void unlockPasswordNote()}
    >
      {t("unlockNote", $preferencesStore.language)}
    </button>
  </div>
{:else if sourceMode}
  <textarea
    bind:this={sourceTextarea}
    class="editor-source-input"
    value={draftContent}
    spellcheck="false"
    aria-label={t("sourceMode", $preferencesStore.language)}
    on:input={(event) => scheduleContentSave(event.currentTarget.value)}
  ></textarea>
{:else}
  {#if MarkdownEditor}
    <svelte:component
      this={MarkdownEditor}
      value={draftContent}
      noteId={note.id}
      onChange={scheduleContentSave}
      bindEditor={bindMarkdownEditor}
    />
  {:else}
    <div class="markdown-editor-host markdown-editor-host--loading"></div>
  {/if}
{/if}
```

Render dialog:

```svelte
{#if protectionOpen && note}
  <NoteProtectionDialog
    currentMode={note.protection_mode}
    onSave={(mode, password) => void saveProtection(mode, password)}
    onClose={() => (protectionOpen = false)}
  />
{/if}
```

- [ ] **Step 7: Add CSS**

Add to `web/app/src/lib/styles/global.css`:

```css
.editor-protected-state {
  display: grid;
  gap: 0.75rem;
  padding: 2rem 0;
  color: var(--text-secondary);
}

.note-protection-dialog__modes {
  display: grid;
  gap: 0.5rem;
  margin: 1rem 0;
}

.note-protection-dialog__modes label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
```

- [ ] **Step 8: Run frontend checks**

Run:

```bash
cd web/app
npm run check
npm test -- src/lib/crypto/noteEncryption.test.ts src/lib/markdown/render.test.ts
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add web/app/src/lib/components/editor/NoteProtectionDialog.svelte web/app/src/lib/components/editor/EditorPane.svelte web/app/src/lib/styles/global.css web/app/src/lib/stores/preferences.ts
git commit -m "Add note protection editor UI"
```

## Task 7: Final Integration Verification

**Files:**
- Modify only if verification reveals a defect in files touched by Tasks 1-6.

**Interfaces:**
- Consumes all prior tasks.
- Produces a verified branch ready for review.

- [ ] **Step 1: Run all Go tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run frontend tests and type checks**

Run:

```bash
cd web/app
npm test
npm run check
npm run build
```

Expected: all PASS.

- [ ] **Step 3: Verify no old API output remains**

Run:

```bash
rg -n "json:\"is_locked|IsLocked|FieldIsLocked|SetIsLocked|FieldPassword|SetPassword" internal ent web/app/src
```

Expected: no matches.

- [ ] **Step 4: Start the dev server**

Run:

```bash
cd web/app
npm run dev -- --port 5173
```

Expected: Vite prints a local URL such as `http://localhost:5173/`. Keep the server running until manual verification is complete.

- [ ] **Step 5: Manual code-group verification**

In the browser:

1. Open a note with a `::: code-group` block.
2. Click the local `Source` button inside the code group.
3. Change a tab label, save, and confirm the whole note did not switch to global source mode.
4. Confirm the rendered tab label updates.

- [ ] **Step 6: Manual protection verification**

In the browser:

1. Create a note with body text.
2. Set protection to Access password.
3. Reload the app and confirm the body is redacted until password unlock.
4. Set protection to Encrypt content.
5. Reload the app and confirm the body requires local password decrypt.
6. Confirm the database row has empty `content` for the encrypted note and non-empty `encrypted_content`.

- [ ] **Step 7: Record verification outcome**

If verification passes without code changes, do not create an empty commit. If verification reveals a defect, return to the task that introduced that defect, apply the fix there, rerun that task's tests, and commit using that task's explicit `git add ...` file list.
