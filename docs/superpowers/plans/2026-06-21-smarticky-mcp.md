# Smarticky MCP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a secure Smarticky MCP provider with LazyCat app-to-app support, per-user external tokens, note creation, and note image generation.

**Architecture:** `/mcp` authenticates every MCP call into a Smarticky user context, then delegates note operations to focused internal services. Token and image download management stay on normal JWT-protected REST endpoints under `/api/mcp/*`.

**Tech Stack:** Go, Echo, Ent, SQLite, official `github.com/modelcontextprotocol/go-sdk/mcp`, Svelte settings UI, LazyCat LPK v2 metadata.

## Global Constraints

- LazyCat header trust is disabled unless `SMARTICKY_TRUST_LAZYCAT_HEADERS=true`.
- External MCP access requires a token bound to a Smarticky note user.
- Locked notes do not expose content or image generation through MCP.
- The first release exposes list/search/get/create/image tools only; no update/delete tools.
- Image generation returns a protected download URL instead of base64 content.

---

### Task 1: Data Model

**Files:**
- Modify: `ent/schema/user.go`
- Create: `ent/schema/mcptoken.go`
- Create: `ent/schema/mcpimage.go`
- Generate: `ent/*`

**Steps:**
- [ ] Add optional unique `lazycat_uid` to `User`.
- [ ] Add `MCPToken` with `name`, `token_hash`, `last_used_at`, timestamps, and owner edge to `User`.
- [ ] Add `MCPImage` with `filename`, `path`, `content_type`, `size`, timestamps, and owner edge to `User`.
- [ ] Run `go generate ./ent`.
- [ ] Run `go test ./ent/...`.

### Task 2: MCP Auth And User Token API

**Files:**
- Create: `internal/mcp/auth.go`
- Create: `internal/handler/mcp_token.go`
- Modify: `cmd/server/main.go`
- Test: `internal/handler/mcp_token_test.go`

**Steps:**
- [ ] Implement bearer-token hashing, token generation, constant-time lookup, and LazyCat header mapping.
- [ ] Add `GET /api/mcp/tokens`, `POST /api/mcp/tokens`, and `DELETE /api/mcp/tokens/:id`.
- [ ] Return the plaintext token only once from create.
- [ ] Add tests for token list/create/revoke and cross-user ownership.

### Task 3: Note Service Boundary

**Files:**
- Create: `internal/notes/service.go`
- Modify: `internal/handler/note.go`
- Test: `internal/notes/service_test.go`

**Steps:**
- [ ] Move reusable note list/search/get/create behavior into a small service.
- [ ] Preserve existing REST response behavior.
- [ ] Add tests for user isolation, search filters, create ownership, and locked note redaction helpers.

### Task 4: Share Image Service

**Files:**
- Create: `internal/shareimage/service.go`
- Create: `internal/handler/mcp_image.go`
- Modify: `Dockerfile`
- Test: `internal/shareimage/service_test.go`

**Steps:**
- [ ] Implement PNG rendering for `classic`, `paper`, and `night` themes and `story`/`square` ratios.
- [ ] Store generated files under the Smarticky data directory.
- [ ] Add `GET /api/mcp/images/:id` as a JWT-protected owner-checked download endpoint.
- [ ] Add CJK font availability to the runtime image.
- [ ] Add tests that generated PNGs are non-empty and image downloads reject other users.

### Task 5: MCP Endpoint And Tools

**Files:**
- Create: `internal/mcp/server.go`
- Modify: `cmd/server/main.go`
- Test: `internal/mcp/server_test.go`

**Steps:**
- [ ] Add `/mcp` with the official MCP Go SDK streamable HTTP handler.
- [ ] Register `smarticky_list_notes`, `smarticky_search_notes`, `smarticky_get_note`, `smarticky_create_note`, and `smarticky_generate_note_image`.
- [ ] Ensure every tool receives the authenticated user id from the request context.
- [ ] Test external token auth, LazyCat header auth, locked note redaction, create-note ownership, and image generation result shape.

### Task 6: Frontend Token Management

**Files:**
- Modify: `web/app/src/lib/api/types.ts`
- Create: `web/app/src/lib/api/mcp.ts`
- Modify: `web/app/src/lib/components/settings/ProfilePanel.svelte`
- Modify: `web/app/src/lib/stores/preferences.ts`

**Steps:**
- [ ] Add a profile section for MCP tokens.
- [ ] Let users list, create, copy, and revoke tokens.
- [ ] Show newly-created plaintext token only once.
- [ ] Keep copy concise in Chinese and English locale maps.

### Task 7: LazyCat Packaging And Docs

**Files:**
- Create: `package.yml`
- Create: `lzc-manifest.yml`
- Create: `lzc-build.yml`
- Create: `resources/mcp-providers/smarticky/mcp.yml`
- Create: `icon.png`
- Modify: `README.md`
- Modify: `ENVIRONMENT_CONFIG.md`

**Steps:**
- [ ] Add LPK v2 metadata with `min_os_version: 1.5.2`.
- [ ] Export MCP provider resource with `endpoint: /mcp`.
- [ ] Document LazyCat user mapping and external token behavior.
- [ ] Create a valid 512x512 PNG icon.

### Task 8: Verification, Commit, Tag, Push

**Files:**
- All modified files.

**Steps:**
- [ ] Run `gofmt` on Go files.
- [ ] Run `go mod tidy`.
- [ ] Run `go test ./...`.
- [ ] Run frontend build if the Svelte UI changed.
- [ ] Review `git diff`.
- [ ] Commit with a feature message.
- [ ] Create the next version tag after inspecting existing tags.
- [ ] Push branch and tag.
