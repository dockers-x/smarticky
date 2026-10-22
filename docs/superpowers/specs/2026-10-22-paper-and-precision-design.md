# Paper & Precision UI/UX

## Outcome

Keep Smarticky a calm, paper-first notebook while making its controls precise,
touch-friendly, accessible, and consistent. Preserve existing notes, Markdown,
encryption, folders, tags, whiteboards, exports, sharing, and account features.

## Design contract

- Content remains warm paper with reader-serif typography; chrome uses the platform UI font.
- Preserve the orange action accent and existing light/dark themes.
- Desktop retains efficient browsing and editing; mobile uses one primary surface at a time.
- Details stay closed by default. Settings hold low-frequency administration.
- Controls have readable labels, visible keyboard focus, instant press feedback, and adequate touch targets.
- Menus and dialogs must close predictably, manage focus, and remain usable on short screens.
- Save status and async failures must be truthful and discoverable.
- Safe-area insets, narrow screens, long titles, landscape, and the on-screen keyboard must not hide essential actions.
- Keyboard navigation and frequent writing actions do not receive ornamental motion.
- Occasional overlays may transition transform/opacity for 150–250 ms using existing easing tokens.
- Respect reduced motion, reduced transparency, and increased contrast.
- Reuse existing dependencies and APIs; do not introduce a motion framework or redesign storage.

## Acceptance

Run frontend type/accessibility checks, existing and targeted tests, the production
asset build, and Go tests. Inspect real screenshots and exercise core flows at
375 px, 390 px, 768 px, and desktop widths in light/dark modes. Cover empty,
populated, selected, editing, settings, confirmation, and failure states where relevant.
Release the next unused minor tag after checking remote tags, update release
documentation, commit all source and embedded build assets, push main and the tag,
then read back the remote refs and report the CI state honestly.

## Prior design evidence

Nowledge CLI search: `smarticky UI UX 纸张 编辑器 版本 发布`; mode
`fast_bm25_vector`; scope `default`; filters unavailable, no explicit filters supplied.
The current source and rendered UI remain authoritative.

| Rank | Memory ID | Title | Score | Applied constraint |
| --- | --- | --- | --- | --- |
| 1 | 25e8e1c1-1227-450e-bab5-12d8ca3cb9ff | Smarticky UI 设计红线：反工具型工作台模式 | 0.9762768663832477 | Keep details closed; administration in settings; mobile list/editor single surface. |
| 2 | 74bd5bc9-2352-49e9-80e8-cce6907db09c | Smarticky 统一视觉语言：全界面 Editorial Knowledge Workspace 方向落地 | 0.9184576156998469 | Preserve warm paper, ink, orange accent, and a coherent authentication experience. |

Graph visualization was skipped: the CLI-only remote context cannot establish
the authenticated browser owner/Space equivalence required for graph access.
