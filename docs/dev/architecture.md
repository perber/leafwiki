# Architecture

See [source-map.md](./source-map.md) for what lives where, and [processes.md](./processes.md) for how the core workflows run end-to-end. Decisions with lasting rationale are recorded as ADRs in [docs/adr/](../adr/) — this document describes the *current shape* of the system; the ADRs explain *why* it's shaped that way.

## Guiding principles (from CONTRIBUTING.md)

- Markdown stored on disk
- Minimal operational complexity
- Single binary deployment
- Explicit structure over hidden automation
- Self-hosting friendly

These principles show up directly in the architecture below (filesystem-as-source-of-truth, no external services, polling instead of a push-connection stack).

## Backend layering

```
cmd/leafwiki
    └── internal/wiki                 (composition root)
            ├── internal/wiki/<domain>        (route + use-case slices)
            │       ├── internal/http, internal/http/middleware/*   (framework glue)
            │       ├── internal/core/auth                          (needed by nearly every slice)
            │       ├── internal/core/shared/errors                 (LocalizedError convention)
            │       └── internal/<domain-service>                   (links/tags/properties/search/…)
            ├── internal/wiki/pagesave                               (cross-cutting side-effect bus)
            └── internal/<domain-service>*
                    └── internal/core/*                              (tree, markdown, revision, shared)
```

- **`internal/core/shared`, `core/shared/errors`, `core/tree`, `core/markdown`** are the true shared/core packages, imported by nearly everything.
- **`internal/core/auth`** is the auth "hub" — imported by almost every `internal/wiki/*` route package for `RequireAuth`/`RequireAdmin` wiring.
- **`internal/wiki/pagesave`** is the one place that knows about *all* post-save side effects (revision, links, tags, properties, search). It's imported by `internal/wiki/pages` and `internal/wiki/revisions`.
- **Leaf packages** (no internal imports besides `core/shared*`): `core/markdown`, `core/excerpt`, `snapshot`, `branding` (the domain package, not its route wrapper).
- No import cycles exist; the layering `wiki/<domain>` → `<domain>` → `core/*` is respected everywhere.

## Storage: bifurcated, not a single repository layer

`internal/core/tree` and `internal/core/revision` write directly to disk (markdown + JSON blobs via `os.WriteFile`). `internal/links`, `internal/tags`, `internal/properties`, `internal/search` each keep a per-wiki SQLite database. The filesystem is the source of truth; every SQLite index is derived and fully rebuildable from it via resync. See [ADR-0001](../adr/0001-filesystem-source-of-truth.md).

## Auth: single-tier in this repo, two-tier only at the hosted layer

Within this repo there is one auth system (`internal/core/auth`): JWT access+refresh tokens, `SessionStore`, `UserStore`, role-based middleware (`RequireAuth`/`RequireAdmin`/`RequireEditorOrAdmin`). An optional reverse-proxy header auth layer (`InjectRemoteUser`) exists as an alternate front door for self-hosters running behind an SSO proxy — it still resolves against the same `UserService`, it does not introduce a second auth system. See [ADR-0005](../adr/0005-reverse-proxy-header-auth.md).

`AuthService.userService` is the one piece of otherwise-static DI wiring that can change after construction: a live restore hot-swaps it (`ReplaceUserStore`) to point at a freshly-restored `users.db`, guarded by a `sync.RWMutex` that covers only that field (`sessionStore` is never swapped). See [ADR-0009](../adr/0009-restore-hot-swap-and-write-gate.md).

The "two-tier auth" architecture referenced in prior planning notes belongs to the separate `leafwiki-hosted` repository (a control-plane layer that runs on top of individual instances), not to this repo.

## Write-gate: a global maintenance-mode middleware

`internal/http/middleware/maintenance.WriteGateMiddleware` is the one piece of HTTP middleware registered globally, ahead of every domain's routes — it 503s any non-GET/HEAD/OPTIONS request while a live restore is swapping files (`internal/restore.WriteGate`, engaged/disengaged by `internal/restore.Manager`). Nil-safe: instances without `--snapshot` enabled never register it, so it's zero-cost there. See [ADR-0009](../adr/0009-restore-hot-swap-and-write-gate.md) for why a blocking gate was chosen over a request queue.

## Error handling convention

`internal/core/shared/errors.LocalizedError{Code, Message, Template, Args, Cause}` is the standard shape for domain errors, surfaced via a `respondWith<Domain>Error` helper in each `internal/wiki/<domain>/errors.go`. Eleven of twelve domain slices follow this. **`internal/backup`/`internal/wiki/backup` do not** — they hand-roll a parallel `BackupErrorDetail` struct and use raw `fmt.Errorf`/`errors.New` throughout the domain package instead of `LocalizedError`. Treat this as a known deviation, not a pattern to copy for new domains.

## Frontend layering

```
App.tsx
  └── features/router/router.tsx (route table) → lazy-routes.tsx (code-split feature pages)
        └── layout/AppLayout.tsx (shell: sidebar, toolbar, dialogs, hotkeys)
              └── feature page components (PageEditor, PageViewer, PageHistoryPage, ...)
                    ├── feature-local store (pageEditorStore, viewerStore, ...)
                    ├── global stores (useTreeStore, useConfigStore, useDialogsStore, ...)
                    └── lib/api/* (typed fetch client)
```

`components/ui/*` and other shared presentational components have zero store dependencies — correctly leaf-level.

**Cross-store coupling is deliberate, not accidental.** Stores call each other's `getState()` directly rather than components orchestrating cross-feature effects: `pageEditorStore.savePage()` reaches into `useTreeStore` (reload/patch), `useViewerStore` (keep an open viewer in sync), `useConfigStore` (feature flags), `useProgressbarStore` (loading indicator), and `useLinkStatusStore` (refresh backlinks). This is documented in code as an intentional "stores talk to stores" pattern. See [ADR-0006](../adr/0006-zustand-feature-local-stores.md).

## Refactoring backlog

These are known-debt items surfaced during a codebase review (2026-07-07). None require a large architecture change — they're localized cleanups. Listed roughly by impact.

### Backend

1. **`MarkdownRefactorEngine` has three divergent rewrite mechanisms, not one tokenizer pass** (`internal/links/link_refactor.go`):
   - `Rewrite()` — AST-based candidate detection (goldmark) *plus* a hand-written byte-offset re-scanner, because goldmark doesn't expose reliable raw offsets for link destinations.
   - `RewriteWikiLinksPrecompiled` — a wholly separate pure-regex mechanism for `[[Title]]` wikilinks.
   - `RewriteRelativeLinksForPathChange` — a near-clone of `Rewrite()` with different semantics for "the current page itself moved."
   A single refactor/rename operation parses the same page content 2-3 times through different code paths (`internal/wiki/pages/refactor.go`). A unifying single-pass tokenizer (one AST walk classifying both link kinds, emitting byte-accurate replacements) would remove an entire mechanism. Known asymmetry caused by this split: `rewritePathChangedSubtree` never invokes the wikilink rewriter, so path-hint wikilinks inside a moved-but-not-renamed subtree may not get corrected — not confirmed as a live bug, but worth a follow-up look before any tokenizer rewrite.
2. **`internal/backup`/`internal/wiki/backup` violate the `LocalizedError` convention** — see above. Fixing this means introducing `LocalizedError` usage in the domain package and replacing `BackupErrorDetail` with the shared type.
3. **`internal/wiki/wiki.go` is a ~750-line composition-root god-object** — DI wiring, `buildXRoutes` for 10+ domains, the sync/async filesystem-reload implementation, and lifecycle (`Close`, `EnsureWelcomePage`) all in one file/type. Splitting the reload/resync logic out (it already half-lives in `internal/wiki/resync` for the job/state part) and moving route wiring into a dedicated file would reduce blast radius for unrelated changes.
4. **`internal/core/tree` concentrates too much in two files** — `tree_service.go` and `node_store.go` together cover in-memory indexing, locking, optimistic-version checks, CRUD, path/permalink resolution, and filesystem reconstruction. Natural split: index maintenance vs. CRUD orchestration vs. path resolution.
5. **Route-wrapper test coverage gap** — `internal/wiki/assets`, `internal/wiki/backup`, `internal/wiki/health`, `internal/wiki/importer`, `internal/wiki/properties` have zero `_test.go` files, even though their underlying domain packages are well tested. Inconsistent with `pages`, `search`, `tags`, `resync`, `pagesave`, which all have route-layer tests.
6. **Two empty, untracked directories**: `internal/accessmode`, `internal/wiki/accessmode`. Safe to delete.

### Frontend

1. **Duplicated rename/refactor flow** — `usePageEditorStore.savePage()` and `TreeNodeActionsMenu.handleRenamePage()` each independently implement `previewPageRefactor()` → `confirmPageRefactor()` → `applyPageRefactor()` → sync-tree/viewer. They've already drifted once (the tree-menu path blocks renaming a page open in the editor; the in-editor path doesn't need that guard). A shared `useRenamePageFlow()` hook would remove the duplication and the drift risk.
2. **`stores/tree.ts` has an open `// FIXME: a better error handling is required here`** in `reloadTree()` — errors are stringified into state with no retry/backoff and no network-vs-server distinction.
3. **Empty orphan directory** `src/features/accessMode/` — no files, no git history. Delete, or stub if a real feature is actually planned there.
4. **No TanStack Query anywhere yet** — confirmed absent from `package.json` and unreferenced in `src/`. ~15 stores each hand-roll near-identical `isLoading`/`error`/`AbortController` plumbing. This is the most-repeated structural pattern in the frontend and the most obvious migration candidate, but as of 2026-07 it is 100% unstarted, not even a partial pilot on one store.
5. **Resync polling is a bespoke `for(;;) { await sleep(800) }` loop** (`stores/resync.ts`) with manual error-counting — the only "live progress" UI in the app, with no shared polling/streaming abstraction. Fine as-is per [ADR-0004](../adr/0004-resync-progress-via-polling.md); flagged here only because it's the one place a future SSE migration would touch.
6. **i18n is English-only** despite `react-i18next` being fully wired up — worth flagging if multi-language support is an assumed near-term feature versus just scaffolding.

None of the above are blocking or urgent; treat this list as a backlog to pull from opportunistically, not a roadmap.
