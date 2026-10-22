# Paper & Precision Implementation Plan

> **For agentic workers:** Use the supplied design-engineering, Apple interaction, and animation skills. Work only within assigned files; the integrator owns shared tokens, global styles, build assets, and release operations.

**Goal:** Preserve paper-first writing while improving usability, mobile navigation, interaction quality, and accessibility throughout Smarticky.

**Architecture:** Keep the Svelte app and Go embedded-asset architecture. Refine shared visual tokens centrally and keep behavioral changes within existing components. Independent workers own editor, workspace navigation, and settings/dialog components; the integrator validates their combined behavior in a real browser.

**Tech Stack:** Svelte 5, TypeScript, CSS, Vite, Vitest, Go, existing Markdown/editor dependencies.

**Spec:** `docs/superpowers/specs/2026-10-22-paper-and-precision-design.md`

## Global Constraints

- Preserve data formats, APIs, paper typography, light/dark themes, and existing capabilities.
- No new runtime dependencies, motion frameworks, branches, or unrelated refactors.
- Workers do not edit shared global CSS, tokens, translations, release files, or generated assets.
- Use existing translation helper and supply inline translated labels where required.
- Only the integrator commits and pushes after verification.

## Review Focus

- Mobile navigation must not hide the list or trap the user in editing.
- Focus must return sensibly after dismissal, including Escape and backdrop clicks.
- Menus and controls must fit long localized text and narrow/landscape screens.
- Save and failure indicators must reflect actual store state without risking unsaved notes.
- Repeated keyboard actions and reduced-motion settings must avoid unnecessary movement.

## Tasks

- [x] Editor: refine command grouping, note actions, contextual controls, save status, paper readability, mobile editing, and detail affordances. Own editor pane, inspector, and Markdown editor.
- [x] Workspace: refine sidebar, mobile navigation, note list/cards, and workspace responsive behavior. Own workspace components except index/graph/calendar and browser panes unless necessary.
- [x] Settings/dialogs: refine settings navigation, focus/dismissal, confirmation, and short-screen layouts. Own settings components and common dialog host.
- [x] Foundation: unify materials, typography, touch sizing, focus, motion, auth screens, safe areas, and visual hierarchy in shared styles/tokens.
- [x] Verification: run check/tests/build and Go tests; exercise keyboard, narrow, desktop, light/dark, and overlay flows; independently review the diff and fix regressions.
- [x] Release preparation: verify remote tags and main, update version/changelog to v0.8.0, and rebuild embedded assets. Commit, atomic main/tag push, remote readback, and CI inspection follow this record and are reported in the release handoff.

## Verification Record — 2026-10-23

- `npm --prefix web/app run check`: 0 errors, 0 warnings.
- `npm --prefix web/app test`: 24 test files, 152 tests passed.
- `npm --prefix web/app run build`: passed; embedded assets regenerated and normalized.
- `go test ./...` and a version-injected server build: passed.
- `git diff --check`: passed before release staging.
- Browser checks used the production Go binary and an isolated temporary database, not user data. The final fresh browser session reported no uncaught application errors.
- Verified note list and editor bounds at 1440×960, 1024×768, 768×1024, 390×844, 375×667, and 844×390; no document-level horizontal overflow. Inspected light/dark, Chinese/English, populated/empty/error, mobile More, settings, and overlay states across the review.
- Verified focus trapping, Escape, focus return, search shortcuts, retry after a forced save failure, and successful sign-in after authentication failure.
- With an 844px layout viewport and simulated 380px VisualViewport height, the editor remained within 380px, its writing surface remained scrollable, and source text stayed at 16px.
- On the production build, inserting text then immediately completing in 4.9ms persisted the final text; switching notes in 2.9ms persisted the previous note without modifying the destination note. Milkdown's delayed update callback no longer serves as the authoritative flush source.
- Verified cancellable busy image export, a stalled authenticated-image timeout recovering after approximately five seconds, and a generated 2160×3658 PNG. Mobile share controls measured 44px high after their entrance transition.

## Evidence Limits

- Responsive browser emulation and the simulated VisualViewport are not physical iOS/Android soft-keyboard or notch tests; those remain device-level follow-up checks.
- Headless Chromium reports neither fine nor coarse pointer; narrow-screen target sizes were measured, while the desktop fine-pointer compact-row rule was also reviewed in source.
- Existing large-chunk build warnings remain for the editor, diagrams, and whiteboard dependencies; no bundle-size improvement is claimed.
- Failed drafts are retained within the current page session, not as a new offline persistence feature.
- The existing tag workflow creates a draft GitHub Release. Tag publication does not imply completed CI, published release assets, or deployment.
