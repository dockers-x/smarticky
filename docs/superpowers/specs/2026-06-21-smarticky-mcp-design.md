# Smarticky MCP Design

## Status
Accepted on 2026-06-21.

## Goal
Expose Smarticky notes through an MCP endpoint that works inside LazyCat app-to-app access and outside LazyCat with per-user tokens.

## Scope
- Add `/mcp` as the Streamable HTTP MCP endpoint.
- Add tools for listing, searching, reading, creating notes, and generating share images.
- Add per-note-user MCP tokens for non-LazyCat callers.
- Add LazyCat provider export files so agents can discover Smarticky as an MCP provider.
- Keep note updates, deletion, attachment upload, and password unlock outside the first MCP release.

## Authentication
MCP requests resolve to a Smarticky note user before any tool can run.

LazyCat mode is enabled only when `SMARTICKY_TRUST_LAZYCAT_HEADERS=true`. In that mode Smarticky accepts forwarded LazyCat identity headers from the LazyCat runtime:
- `X-HC-User-ID` is required.
- `X-HC-SOURCE` must be `app:self` or start with `app:`.
- The LazyCat user id must map to `users.lazycat_uid`.

External mode uses `Authorization: Bearer <token>`. Tokens are bound to a Smarticky user, stored only as SHA-256 hashes, and can be listed, created, and revoked by the current logged-in user through `/api/mcp/tokens`.

## User Mapping
Add `lazycat_uid` to `User` as an optional unique field. The first release does not auto-create Smarticky users from LazyCat users; missing mappings return unauthorized errors. This avoids accidentally binding external callers to the wrong note account.

## MCP Tools
- `smarticky_list_notes`: list the current user's non-deleted notes with pagination.
- `smarticky_search_notes`: search the current user's non-deleted notes by title or content.
- `smarticky_get_note`: return one owned note by id.
- `smarticky_create_note`: create a note for the current user with `title`, `content`, and optional `color`.
- `smarticky_generate_note_image`: render a PNG share image from an owned note id, or from explicit `title` and `content`.

Locked notes never expose content through MCP and cannot be rendered to images. MCP tools return metadata with `is_locked=true` instead.

## Share Image Generation
Move share image rendering into a backend service so MCP can generate real PNG files without relying on browser Canvas. The service mirrors the existing frontend layout: `classic`, `paper`, and `night` themes plus `story` and `square` ratios. Long story images expand vertically for long content.

Image generation writes PNG files under Smarticky's data directory and returns a protected `/api/mcp/images/:id` download URL. The download endpoint requires the same current user's JWT and verifies image ownership. MCP tool results include the URL, filename, content type, and byte size.

## LazyCat Packaging
Use LPK v2 files:
- `package.yml`
- `lzc-manifest.yml`
- `lzc-build.yml`
- `resources/mcp-providers/smarticky/mcp.yml`

`min_os_version` is `1.5.2` because `.lzcx` app interconnect and MCP resource exports require it. Smarticky as a provider does not need `lzcapp.user_delegate`; caller apps need that permission when they access Smarticky.

## Security Notes
- Do not trust LazyCat identity headers unless the explicit environment gate is enabled.
- Do not log MCP bearer tokens or raw token values.
- Do not store token plaintext.
- Do not allow cross-user note lookup, image lookup, or token management.
- Cap MCP list/search limits and note/image input sizes.

## Verification
- Go tests cover token creation/revocation, LazyCat header mapping, external token auth, cross-user isolation, locked note behavior, create-note ownership, and image generation ownership.
- Full verification command is `go test ./...`.
