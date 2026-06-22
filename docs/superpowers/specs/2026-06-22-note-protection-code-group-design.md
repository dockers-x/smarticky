# Note Protection and Code Group Editing Design

Date: 2026-06-22
Status: Proposed

## Summary

Smarticky needs two related but separate improvements:

- Code group source editing should be local to the selected code group. Clicking `Source` inside a rendered code group must not switch the whole note into global source mode.
- Note protection should become an explicit protection model with `none`, `password`, and `encrypted` modes. The default is `none`. Password protection is an access gate. Encrypted mode is client-side zero-knowledge encryption for note body content: the server stores ciphertext and metadata, not the user password or plaintext body.

The clean target is to remove `is_locked` and legacy `password` from the public note model. Migration may read those legacy columns to seed the new fields, but the final API, frontend types, MCP output, and new business logic should use `protection_mode` and `content_redacted` instead. The migration must not rebuild the notes table, drop legacy columns, or delete user data.

## Goals

- Replace the code group `Source mode` button with local code group source editing.
- Let users edit the rendered code group tab/content source without leaving the rich editor context.
- Add a persisted note protection mode with values `none`, `password`, and `encrypted`, defaulting to `none`.
- Support access-password protected notes using the existing Argon2 verification pattern.
- Support true encrypted notes where the browser encrypts/decrypts body content and the server cannot recover plaintext.
- Make encrypted-note limitations explicit for search, MCP, share image export, and server-side rendering.
- Migrate existing locked notes to `protection_mode="password"` without dropping legacy columns or rebuilding the notes table.

## Non-Goals

- No encryption of note title, tags, folder, color, timestamps, attachments, or whiteboard rows in the first stage.
- No server-side full-text search inside encrypted note bodies.
- No MCP access to encrypted note bodies.
- No server-side PNG generation from encrypted note bodies.
- No shared-note or multi-user key sharing model.
- No password recovery for encrypted notes.
- No drag-and-drop or visual tab manager for code groups in this stage.

## Current Context

Code group rendering and editor previews are already implemented under `web/app/src/lib/markdown/codeGroups.ts` and `web/app/src/lib/markdown/editorCodeGroups.ts`. The editor plugin renders a code group widget and hides the underlying Markdown source in the Milkdown document. The current `Source mode` button calls a `requestSourceMode` callback, which flows through `MarkdownEditor.svelte` into `EditorPane.svelte` and flips the whole note into the raw Markdown textarea.

The note schema currently has legacy `password` and `is_locked` fields. The handler accepts those fields in note updates, hashes passwords with Argon2, and has a `/verify-password` endpoint. This can represent access-password protection, but it cannot represent zero-knowledge encrypted content.

## Product Model

### Code Group Editing

A code group is a structured Markdown block inside a note. Its local source is the text from the opening `::: code-group` or `::: code-tabs` line through the matching closing marker.

Clicking `Source` on a code group opens an inline local editor for that block only. The user can save or cancel. Saving replaces only that code group's Markdown range in the note content, then the rich editor refreshes its preview. Global source mode remains available from the editor toolbar and keyboard shortcut.

### Note Protection

Each note has one protection mode:

```ts
type NoteProtectionMode = "none" | "password" | "encrypted";
```

- `none`: body content is stored as plaintext in `notes.content`.
- `password`: body content is stored as plaintext, but the app requires a password before showing/editing the content in protected flows. This is not database encryption.
- `encrypted`: body content is encrypted in the browser. The server stores ciphertext plus encryption metadata and does not receive the password or plaintext body.

In the first stage, encrypted mode protects the note body only. Title, tags, folder, color, attachments metadata, whiteboard metadata, and timestamps remain visible to the server.

## Database Shape

Add explicit protection fields to the note schema:

```go
field.Enum("protection_mode").
    Values("none", "password", "encrypted").
    Default("none")

field.String("protection_password_hash").
    Optional().
    Sensitive()

field.Text("encrypted_content").
    Optional().
    Sensitive()

field.String("encryption_alg").
    Optional()

field.String("encryption_kdf").
    Optional()

field.String("encryption_salt").
    Optional()

field.String("encryption_nonce").
    Optional()
```

Stop exposing the legacy `password` and `is_locked` fields through new public contracts after migration logic can convert existing rows. The physical database columns remain in place as inert legacy columns for safety.

Migration policy:

- Existing rows with legacy `is_locked=false` become `protection_mode="none"`.
- Existing rows with legacy `is_locked=true` become `protection_mode="password"`.
- Existing `password` hash values are copied to `protection_password_hash` when present.
- After conversion, keep the legacy `password` and `is_locked` columns physically present but unused. Do not run `DROP COLUMN`, do not rebuild the `notes` table, and do not clear those legacy values during this feature.
- New writes update `protection_mode` and the new fields only.
- New reads use `protection_mode` and the new fields only. The only allowed read of legacy columns is a guarded backfill step for existing databases.

Encrypted rows store an empty plaintext `content` or leave it as the last plaintext only during a guarded migration step. The target invariant is: when `protection_mode="encrypted"`, `notes.content` must not contain the decrypted body.

## API Shape

Note responses include:

```ts
interface Note {
  id: UUID;
  title: string;
  content: string;
  color: string;
  protection_mode: "none" | "password" | "encrypted";
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

Behavior:

- `GET /api/notes` returns encrypted notes without plaintext `content`; it may include encryption metadata needed for unlock.
- `GET /api/notes/:id` behaves the same unless the note is already decrypted on the client; the server never decrypts encrypted notes.
- `PUT /api/notes/:id` accepts `protection_mode`.
- `password` mode updates accept a password and store only `protection_password_hash`.
- `encrypted` mode updates accept ciphertext and encryption metadata, not a password.
- `none` mode clears protection hash, encrypted payload, and encryption metadata.
- `/verify-password` remains for access-password mode and should reject encrypted mode with a clear error.
- Requests that still send legacy `is_locked` should be rejected as unknown/deprecated fields rather than silently accepted.

Validation:

- `protection_mode` must be one of the three allowed values.
- `password` mode requires a non-empty password when enabling protection.
- `encrypted` mode requires ciphertext, algorithm, KDF, salt, and nonce.
- Switching from `encrypted` to `none` or `password` requires the client to send decrypted plaintext content.
- Switching from `none` or `password` to `encrypted` requires the client to send ciphertext and clear plaintext content from server storage.

## Client Encryption

Use Web Crypto APIs in the browser:

- Key derivation: PBKDF2-SHA-256 with a per-note random salt and a high iteration count chosen during implementation.
- Encryption: AES-GCM with a per-save random nonce.
- Stored metadata: `encryption_alg`, `encryption_kdf`, `encryption_salt`, `encryption_nonce`.
- Stored body: base64 ciphertext in `encrypted_content`.

The password used for encrypted notes is never sent to the server. Unlocking an encrypted note means deriving the key in the browser and attempting AES-GCM decrypt. A wrong password produces a generic unlock failure.

Implementation may choose a versioned metadata string such as `pbkdf2-sha256:310000` so future KDF changes can be introduced without guessing.

## Frontend UI Shape

Add a protection control in the editor details/actions area, not as a primary writing control. The user flow:

1. Open note actions or details.
2. Choose protection mode: None, Access password, Encrypt content.
3. For Access password, enter and confirm password.
4. For Encrypt content, enter and confirm encryption password and see a short warning that password loss cannot be recovered.
5. Save protection settings.

Protected display:

- Password mode: show a locked body state until the access password is verified.
- Encrypted mode: show an encrypted body state until the user enters the encryption password locally.
- After unlock, the editor receives plaintext `draftContent` in memory.
- Saving an encrypted note encrypts the latest body before sending it to the server.

The unlocked plaintext should be kept only in client state while the note is open. It should not be written to localStorage.

## Code Group Local Source Editing

Extend the code group editor plugin boundary so the widget can request a local source edit:

```ts
interface CodeGroupSourceEditRequest {
  sourceID: string;
  startLine: number;
  endLine: number;
  raw: string;
}

type ReplaceCodeGroupSource = (request: CodeGroupSourceEditRequest, nextRaw: string) => void;
```

The widget should render:

- The existing tabbed preview.
- A compact `Source` button.
- When active, a textarea or CodeMirror-backed local source editor.
- `Save` and `Cancel` actions.

Saving validates that the replacement is still a supported `code-group` or `code-tabs` block. If valid, replace the exact source block in the current Markdown and call the editor's normal `onChange` path. If the original range no longer matches because the user edited elsewhere, locate the block by `sourceID` or current raw source signature before applying. If it cannot be found, show a local error and do not overwrite unrelated content.

The global source mode API should no longer be passed into `createCodeGroupEditorPlugin`.

## Search, MCP, and Export Behavior

Search:

- `none` and `password` notes can continue matching title and body, subject to existing redaction rules.
- `encrypted` notes match only title, tags, folder, and timestamps. Body search is not available server-side.

MCP:

- `none` notes may expose content as before.
- `password` notes keep the existing protected-content redaction behavior.
- `encrypted` notes must return metadata only with `content_redacted=true` and `protection_mode="encrypted"`.

Share image export:

- Browser share image generation can work after the user unlocks an encrypted note because the browser has plaintext.
- Server-side image generation must reject encrypted notes or return a metadata-only error until a client-side handoff exists.

Attachments and whiteboards:

- Existing attachment files and whiteboard rows are not encrypted by this change.
- Markdown references inside encrypted body content are protected as part of the encrypted body.
- Attachment and whiteboard metadata remain server-visible.

## Security

- Never send encrypted-note passwords to the server.
- Never log passwords, plaintext decrypted content, ciphertext payloads, or encryption metadata in request logs.
- Do not store decrypted encrypted-note content in localStorage or persistent browser storage.
- Use unique random salt per encrypted note and unique random nonce per encryption save.
- AES-GCM nonce reuse with the same key is forbidden.
- Password-mode errors should not reveal whether a hash exists beyond the user already owning the note.
- Encrypted-mode unlock errors should be generic.
- Keep authorization checks unchanged: users can only access their own note records.

## Testing

Frontend tests:

- Code group `Source` opens local editing and does not call global source mode.
- Saving local code group source replaces only that code group block.
- Cancel leaves note content unchanged.
- Invalid local replacement shows an error and does not mutate content.
- Encrypted note unlock decrypts with the right password and fails generically with the wrong password.
- Saving encrypted note sends ciphertext and metadata, not plaintext content.

Go tests:

- Note schema defaults `protection_mode` to `none`.
- Updating to password mode stores a hash and returns `protection_mode="password"`.
- Existing legacy locked rows migrate to password mode without table rebuild, column drop, or data deletion.
- Note responses, frontend types, and MCP output do not include `is_locked`.
- Encrypted note responses do not include plaintext body.
- Search does not match encrypted body ciphertext or plaintext.
- MCP redacts encrypted notes and includes protection metadata.
- Cross-user unlock and update checks remain enforced.

Manual verification:

- Code group tab switching still works in rendered notes and editor preview.
- Code group local source editing preserves labels like ` ```bash [pnpm]`.
- Global source mode still works from the editor toolbar.
- Password-protected note can be locked, unlocked, edited, and unprotected.
- Encrypted note body is unreadable in the database.
- Reloading the app requires unlocking encrypted notes again.

## Rollout Plan

1. Add schema fields and regenerate Ent.
2. Add migration logic for legacy `is_locked/password` rows without table rebuild or column deletion, then remove legacy fields from public types and new business logic.
3. Extend API types and handlers around `protection_mode`.
4. Add frontend note protection types and store update support.
5. Implement client-side encryption helpers with tests.
6. Add editor protection UI and locked/encrypted body states.
7. Replace code group global source-mode callback with local source editing.
8. Update MCP note views and share image behavior for encrypted notes.
9. Run Go tests, frontend tests, and manual editor verification.

## Open Decisions Resolved

- True encryption means client-side zero-knowledge encryption.
- The first encrypted scope is note body content only.
- `protection_mode` is the source of truth and defaults to `none`.
- `is_locked` and legacy `password` are removed from the public model after migration seeding, while their physical legacy columns remain untouched.
- Encrypted bodies are not server-searchable in this stage.
