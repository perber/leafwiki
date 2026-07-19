# Source Map

Internal reference for navigating the LeafWiki codebase. Two halves: the Go backend (`internal/`, `cmd/`) and the React/TS frontend (`ui/leafwiki-ui/src`). See [architecture.md](./architecture.md) for how these layers relate, and [processes.md](./processes.md) for how the core workflows actually run end-to-end.

## Backend (`internal/`, `cmd/leafwiki`)

`cmd/leafwiki` is the binary entrypoint: flag/env parsing, `slog` setup, wiring `internal/wiki.Wiki` + the HTTP router, starting the server, OS signal handling for live-reload (`signals_unix.go` / `signals_windows.go`).

### Composition root

- **`internal/wiki`** — the composition root. `wiki.go` (~750 lines) builds every service, every use case, every route registrar, and owns the filesystem-reload/resync orchestration. Imports nearly all other first-party packages by design (it's the DI layer for `main`).

### Core/foundational packages (`internal/core/*`)

| Package | Responsibility |
|---|---|
| `core/tree` | The page tree: `NodeStore` (filesystem I/O — markdown+frontmatter read/write, slug rename, move, delete, reconstruct-from-disk) and `TreeService` (in-memory tree, locking, optimistic-version checks, CRUD orchestration). Largest package in the repo. |
| `core/markdown` | Frontmatter parsing/serialization (`MarkdownFile`), reserved-field handling. |
| `core/revision` | Page revision/version history — `FSStore` (content-addressed blobs + JSON revision files on disk) and `Service` (content/structure/asset revisions, coalescing, restore, blob GC, integrity checks). |
| `core/auth` | JWT/session auth core — `AuthService`, `UserService`, `SessionStore`, login-attempt tracking, roles (`RoleAdmin`, `RoleEditor`, `RoleViewer`). |
| `core/assets` | Page asset (image/file upload) storage service. |
| `core/excerpt` | Plain-text excerpt extraction from markdown (used by search/tags). |
| `core/tools` | Admin CLI-style tools (e.g. admin password reset). |
| `core/treemigration` | One-shot schema migration runner for the tree's on-disk format. |
| `core/shared` | Small shared utilities; plus `core/shared/errors` (the `LocalizedError` convention, see [architecture.md](./architecture.md)), `core/shared/htmlutil`, `core/shared/sqliteutil`. |

### Domain services (`internal/*`, flat)

| Package | Responsibility |
|---|---|
| `links` | Wikilink/markdown-link index and rewrite engine — `LinkService`, `LinksStore` (SQLite), `MarkdownRefactorEngine`, sentinel handling for `[[Title]]` links (see [ADR-0002](../adr/0002-wikilink-sentinel-encoding.md)). |
| `tags` | Tag index (SQLite), extracted from page frontmatter/content. |
| `properties` | Arbitrary frontmatter "properties" index (SQLite). |
| `search` | Full-text search index (`SQLiteIndex`). |
| `snapshot` | Full-backup ZIP snapshots — `Manager`/`Scheduler` (mirrors `backup`'s scheduler shape), `createSnapshot` zips `root/`, `assets/`, `branding/`, `branding.json`, `schema.json`, and a `VACUUM INTO` copy of `users.db` (the one backup path that includes the database). Retention pruning keeps the newest N. Correctly uses `LocalizedError` (unlike `backup`). |
| `restore` | Live/offline restore-from-snapshot — `Manager` (validate/stage/swap/rollback state machine, mirrors `snapshot.Manager`'s async-job shape), `WriteGate` (blocks mutating requests during the file swap, no HTTP dependency), `RestoreOffline` (the `leafwiki restore-snapshot` CLI path — same swap primitives, no gate/auth-reopen/resync). See [ADR-0009](../adr/0009-restore-hot-swap-and-write-gate.md). |
| `branding` | Site branding (logo/name/favicon) service + store. `Reload()` re-reads `branding.json` from disk — used by `restore` after a swap. |
| `backup` | Git-based backup — repo init/commit/push, SSH auth, scheduler, conflict handling. Deviates from the `LocalizedError` convention (see [architecture.md](./architecture.md)). |
| `importer` | ZIP-based content import — planner, executor, content transformer, zip extraction. |

### HTTP / use-case layer (`internal/http/*`, `internal/wiki/<domain>`)

- **`internal/http`** — `RouteRegistrar` interface, `NewRouter` (gin engine setup, embedded SPA serving, custom stylesheet, base-path handling).
- **`internal/http/dto`** — response DTOs shared across route packages.
- **`internal/http/middleware/auth`** — `RequireAuth`, `RequireAdmin`, `RequireEditorOrAdmin`, cookie handling, and `reverse_proxy.go` (trusted-header SSO auth, see [ADR-0005](../adr/0005-reverse-proxy-header-auth.md)).
- **`internal/http/middleware/security`** — CSRF middleware/cookie, rate limiter.
- **`internal/http/middleware/maintenance`** — `WriteGateMiddleware`, the gin wrapper around `internal/restore.WriteGate` (503s mutating requests while a restore is in progress); registered once in `router.go`, nil-safe (no-op when snapshot/restore is disabled).
- **`internal/wiki/<domain>`** — one vertical slice per domain (`pages`, `revisions`, `links`, `tags`, `properties`, `search`, `auth`, `assets`, `branding`, `importer`, `backup`, `snapshot`, `restore`, `health`, `resync`), each with `routes.go` + `use_cases.go`/`errors.go`. `internal/wiki/pages` also hosts the link-refactor preview/apply endpoints (`refactor.go`).
- **`internal/wiki/pagesave`** — not HTTP-facing; the page-save side-effect orchestrator shared by the `pages`/`revisions` use cases (see [processes.md](./processes.md#page-save--write-flow)).
- **`internal/wiki/resync`** — admin filesystem-resync trigger/status routes plus the `ResyncJob` state machine.
- **`internal/wiki/restore`** — admin live-restore trigger/status/self-restart routes wrapping `internal/restore.Manager`. Only wired up (in `cmd/leafwiki/main.go`) when `--snapshot` is enabled, since it restores one of the instance's own snapshots.

### Known dead weight

`internal/accessmode` and `internal/wiki/accessmode` are empty directories, untracked by git, containing no Go files — stale scaffolding from an abandoned feature. Safe to remove.

### Storage split

There is no single "repository layer." Two storage models coexist by design:

- **Filesystem-native, source of truth**: `core/tree` and `core/revision` persist directly to disk (markdown + JSON blobs).
- **SQLite-native, derived/rebuildable**: `links`, `tags`, `properties`, `search` each keep a per-wiki SQLite database, opened via `core/shared/sqliteutil`. These can be fully rebuilt from the markdown tree via resync (see [ADR-0001](../adr/0001-filesystem-source-of-truth.md)).

## Frontend (`ui/leafwiki-ui/src`)

| Path | Responsibility |
|---|---|
| `main.tsx`, `App.tsx` | App entry. `App.tsx` bootstraps auth, config, branding, design mode, then renders the router inside `Suspense`/`ErrorBoundary`. |
| `layout/AppLayout.tsx` | The single shell: sidebar, toolbar, progress bar, dialog manager, hotkey handler, editor title bar, user toolbar, print-mode handling. Wraps every routed page. |
| `components/` | Cross-feature shared UI: `BaseDialog`, `DialogManager`, `ErrorBoundary`, `FormInput`, `HotKeyHandler`, `ListView`, `Page404`, `Pagination`, `RoleGuard`, `TagInputWithSuggestions`, `UnsavedChangesDialog`, `UserToolbar`. |
| `components/ui/` | shadcn/radix primitives only — no domain logic. |
| `features/*` | Feature-sliced modules, one folder per domain: `assets`, `auth`, `backup`, `branding`, `designtoggle`, `editor`, `history`, `imagepreview`, `importer`, `links`, `maintenance`, `page`, `page-switcher`, `preview`, `progressbar`, `router`, `search`, `shortcuts`, `sidebar`, `snapshot`, `tags`, `toolbar`, `tree`, `users`, `viewer`, `wikilinks`. |
| `stores/` | App-wide/global Zustand stores (see below). |
| `lib/api/` | One typed fetch wrapper per backend resource: `pages`, `assets`, `auth`, `backup`, `branding`, `config`, `import`, `links`, `properties`, `resync`, `restore`, `revisions`, `search`, `snapshot`, `tags`, `users`, plus `errors.ts`. |
| `lib/registries/` | `dialogRegistry.ts`, `lazy-dialogs.tsx`, `panelItemRegistry.ts` — decouples triggers from dialog/panel implementations. |
| `lib/shortcuts/` | Keyboard-shortcut definitions consumed by `HotKeyHandler`. |
| `locales/en/*.json` | i18next namespaces (English only — no other locale directories exist despite `react-i18next` being fully wired up). |

### Feature folder highlights

- **`features/editor/`** — `PageEditor.tsx`, `pageEditorStore.ts`, `MarkdownEditor.tsx`/`MarkdownCodeEditor.tsx` (CodeMirror 6), `useAutoSave.ts`, `frontmatter.ts`, `internalLinkCompletion.ts` (wikilink autocomplete), `htmlToMarkdown.ts` + `pasteImageUpload.ts` (rich paste via `turndown`).
- **`features/preview/`** — the single shared Markdown renderer: `MarkdownPreview.tsx` plus `extractTocEntries.ts`, `MarkdownCodeBlock.tsx`, `MarkdownImage.tsx`, `MarkdownLink.tsx`, `MermaidBlock.tsx`, `normalizeMarkdownShoutouts.ts` (custom `[!INFO]`/`[!WARNING]` alerts), `rehypeLineNumber.ts`, `TocSidePanel.tsx`.
- **`features/viewer/`** — `PageViewer.tsx` (read-mode shell, statically imported — see [processes.md](./processes.md#markdown-preview-rendering)), `Breadcrumbs.tsx`, `viewer.ts` store.
- **`features/tree/`** — `TreeView.tsx`, `TreeNode.tsx`, `TreeNodeActionsMenu.tsx` (rename/move/delete/pin context menu), `PinnedSection.tsx`/`PinnedPageItem.tsx`.
- **`features/progressbar/`** — global top-of-page loading bar, decoupled from any specific request via `progressbarStore`.
- **`features/page/`** — all page-mutation dialogs (`AddPageDialog`, `MovePageDialog`, `SortPagesDialog` — the only `dnd-kit` consumer, for sibling reordering — `PageRefactorDialog.tsx`, `PageHistoryPage.tsx`).
- **`features/maintenance/`** — `MaintenanceSettings.tsx`, hosts the filesystem-resync trigger button and progress UI.
- **`features/snapshot/`** — `SnapshotSettings.tsx`, full-backup creation/list/download/delete *and* the restore UI (per-snapshot "Restore" action, confirm dialog, restore+resync progress banner, self-restart recovery banner when a restore reports `needsIntervention`).
- **`features/accessMode/`** — empty directory, no files, no git history. Stray scaffolding, same pattern as backend's `internal/accessmode`.

### State management (Zustand v5, no Redux, no TanStack Query)

Global stores in `src/stores/`:

| Store | Responsibility |
|---|---|
| `editor.ts` | Editor UI preferences (preview visible/stacked, line wrap, auto-save on/off). Persisted. |
| `tree.ts` | The full page/section tree, `byId`/`byPath` indexes, pinned pages, open-node-id set (persisted), `reloadTree()`, `patchNodeVersion()`. |
| `session.ts` | Logged-in user info, token expiry, `logout()`. Persisted. |
| `config.ts` | Server-provided feature flags (`enableRevision`, `enableLinkRefactor`, `gitBackupEnabled`, `httpRemoteUserEnabled`, `authDisabled`, `publicAccess`, ...). |
| `dialogs.ts` | Single active-dialog registry — one dialog open at a time app-wide. |
| `sidebar.ts` | Sidebar visibility, width, mode (tree/search). Persisted. |
| `hotkeys.ts`, `backup.ts`, `branding.ts`, `import.ts`, `users.ts`, `resync.ts`, `snapshot.ts`, `restore.ts` | Feature-specific, app-scoped stores. `restore.ts` polls its own restore-status endpoint, then hands off to `resync.ts`'s existing `getResyncStatus` for the tail end rather than issuing its own resync trigger. |

Feature-local stores (co-located inside `features/*`, not in `src/stores/`):

- `features/editor/pageEditorStore.ts` — the editor's page-editing state (dirty tracking, `savePage()`, `loadPageData()`).
- `features/viewer/viewer.ts` — read-mode page state (`AbortController`-based cancellation).
- `features/progressbar/progressbarStore.ts` — trivial `{ loading, setLoading }`, used as a cross-store signal.
- `features/links/linkstatus_store.ts`, `features/page/pageRefactorDialogState.ts`, `features/page-switcher/pageQuickSwitcher.ts`, `features/designtoggle/designmode.ts`, `features/toolbar/toolbarStore.ts`, `features/tree/treeNodeActionsMenus.ts`.

TanStack Query is **not** a dependency anywhere in `package.json`, and no source file references it. Any migration is still unstarted (see [architecture.md](./architecture.md#refactoring-backlog)).

## Sibling repository: `leafwiki-hosted`

`/home/patrick/Customers/socradev/leafwiki-hosted` is a separate repository implementing the multi-tenant control plane for the hosted offering (`cmd/controlplane`, `data/tenants/`). It runs *on top of* individual LeafWiki instances rather than being part of this repo — see [ADR-0005](../adr/0005-reverse-proxy-header-auth.md) for how the two relate on the auth side.
