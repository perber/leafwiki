# LeafWiki Processes

How the core workflows actually run, end to end, through real code. See [Developer Docs](./README.md) for the index, [LeafWiki Source Map](./source-map.md) for package-by-package
structure, [LeafWiki Architecture](./architecture.md) for the layering/storage/auth model this doc assumes.

## Page save

Entry point: `PUT /api/pages/:id` (update — the one the editor auto-save hits), also
`POST /api/pages` (create), `POST /api/pages/copy/:id` (copy), `DELETE /api/pages/:id`
(delete), `PUT /api/pages/:id/move` (move — see [Tree rename/move](#tree-renamemove)), and revision restore.
All routes registered in `internal/wiki/pages/routes.go`, guarded by
`authmw.RequireEditorOrAdmin()` and `security.CSRFMiddleware`.

Update flow, concretely:

1. `Routes.handleUpdate` (`internal/wiki/pages/routes.go`) binds/validates the JSON body,
   normalizes tags to lowercase (`normalizeTagInputs`), resolves the authenticated user, and
   calls `UpdatePageUseCase.Execute`.
2. `UpdatePageUseCase.Execute` (`internal/wiki/pages/update_page.go`):
   - Validates title/slug (`sharederrors.ValidationErrors`).
   - `uc.tree.GetPage(in.ID)` to snapshot `before` (old title/content/path).
   - Calls `TreeService.UpdateNode(UpdateNodeInput{…})` (`internal/core/tree/tree_mutations.go`), which runs
     inside `withLockedTree` — a single mutex around the whole in-memory tree — and: checks
     optimistic-concurrency version (`checkNodeVersion`, returns a conflict error the frontend
     surfaces as `page_version_conflict`), enforces slug uniqueness under the parent, writes
     content to disk via `store.UpsertContent`/`UpsertContentAndMetadata`/
     `UpsertContentPreservingFrontmatter` (mutually exclusive — callers must not set both
     `PreserveFrontmatter` and `Tags`/`Properties` in the input), renames the on-disk file if the slug
     changed, updates the in-memory title index, and syncs frontmatter.
   - Diffs `before`/`after` to compute `contentChanged`, `titleChanged`, `slugChanged`.
   - Builds a `pagesave.PageSaveEvent` (`internal/wiki/pagesave/event.go`) and calls
     `orchestrator.Run(event)`.
3. The markdown file on disk is the durable write; everything from here on is derived-index
   maintenance.

**The effect-pipeline pattern** (`internal/wiki/pagesave/orchestrator.go`):
- `PageSaveOrchestrator.Run` iterates registered `PageSideEffect`s **in registration order**
  and calls `Apply(event)` on each, synchronously, on the request goroutine — there is no
  async dispatch or queue.
- Registration order in `Wiki.newPageOrchestrator` (`internal/wiki/wiki_routes.go`): search →
  links → revision → tags → properties.
- **Failures do not block the save or roll back the disk write.** Each effect's `Apply` is
  documented as "errors are handled internally (best-effort)"
  (`internal/wiki/pagesave/orchestrator.go`): every effect logs a `Warn` on failure and
  increments `httpmetrics.IncPageSaveSideEffectFailure(operation, effectName)`, then returns
  normally — the orchestrator has no error return path at all. Confirmed by
  `TestPageSaveOrchestrator_RecordsSideEffectDurationAndFailures`
  (`internal/wiki/pagesave/effects_test.go`): a failing effect still lets `Run` complete
  and only asserts the failure metric was recorded. A search-index write failure, for example,
  never fails the `PUT` request — the page save succeeds, the page is just temporarily
  unsearchable until the next successful reindex or resync.
- The five effects (`internal/wiki/pagesave/{search,link,revision,tags,properties}_effect.go`)
  each switch on `event.Operation` (`create`/`update`/`move`/`delete`/`restore`) and call into
  their respective service (`search.SQLiteIndex`, `links.LinkService`, `revision.Service`,
  `tags.TagsService`, `properties.PropertiesService`). Move and delete batch across
  `event.AffectedPages` (the whole subtree); tags/properties are no-ops on move since
  `page_id` is stable and frontmatter doesn't change.
- Metrics are recorded per effect regardless of outcome:
  `httpmetrics.ObservePageSaveSideEffect(operation, effectName, startedAt)`
  (`orchestrator.go`).

Same orchestrator/effect set is reused verbatim by `CreatePageUseCase`, `CopyPageUseCase`,
`DeletePageUseCase`, `EnsurePathUseCase`, `RestoreRevisionUseCase`, and the bulk importer
(`internal/wiki/import_adapter.go`) — one pipeline for every page mutation, not one per
endpoint.

## Revisioning

See [ADR-0003: Revision Coalescing (Fixed Window)](./adr/0003-revision-coalescing-fixed-window.md) for the accepted design and its history
(a debounce-based attempt was tried and reverted). This section verifies that design against
the current code.

1. `pagesave.RevisionSideEffect.Apply` (`internal/wiki/pagesave/revision_effect.go`) decides
   *whether* to record a revision, based on `event.Operation`:
   - `create` → `recordContent`.
   - `update` with `SlugChanged` → `recordStructure` per affected page (except the root page
     of a content-changing rename, which instead gets a content revision).
   - `update` with `TitleChanged && !ContentChanged` → `recordStructure`.
   - `update` with `ContentChanged` → `recordContent`.
   - `move` → `recordStructure` per affected page.
   - `delete` → no revision entry (cleanup handled directly by the use case).
   - `restore` → no-op here; `RestoreRevisionUseCase` already wrote a
     `RevisionTypeRestore` entry itself before running the orchestrator.
2. `recordContent`/`recordStructure` call `revision.Service.RecordContentUpdate` /
   `RecordStructureChange` (`internal/core/revision/service.go`), which both funnel
   into `recordContentUpdateForPage` (`service.go`) for the content path.
3. **Coalescing** (`service.go`), per page:
   - `pageWriteLock(pageID)` — a per-page `sync.Mutex` from a `sync.Map`
     (`service.go`) — is held for the whole record so concurrent saves to the same
     page can't race the coalesce decision.
   - No-op short-circuit: if the new state's `ContentHash` and `ExtraFrontmatterHash` both
     match the previous revision, nothing is written at all — returns the existing revision,
     `changed=false` (`service.go`).
   - `shouldCoalesce(prev, authorID)` (`service.go`) requires **all** of: coalescing
     enabled (`coalesceWindow > 0`), a previous revision exists, `prev.Type ==
     RevisionTypeContentUpdate`, same author, and `elapsed := time.Since(prev.CreatedAt)` with
     `0 <= elapsed <= coalesceWindow`.
   - Default `coalesceWindow` is **5 minutes**, configured via `--revision-coalesce-window` /
     `LEAFWIKI_REVISION_COALESCE_WINDOW` (`cmd/leafwiki/flags_frontend.go`, wired through
     `internal/wiki/wiki.go`); `0` disables coalescing entirely.
   - On coalesce: overwrites the *same* revision record in place (reuses `prev.ID`), saves the
     new content blob, and garbage-collects the old blob if the hash changed
     (`store.DeleteContentBlobIfUnreferenced`, `service.go`) — no new revision row.
   - Otherwise: saves a new content blob, creates a new `Revision` via `newRevision`, persists
     it, and prunes old revisions (`pruneAfterSave`).
4. Ratio check: the editor's auto-save debounce is 2s (`useAutoSave.ts`, see
   [Editor auto-save](#editor-auto-save)); a 5-minute default coalesce window means a continuous editing
   session groups many auto-saves into one revision, only splitting into a new revision once
   editing pauses for >5 minutes or resumes >5 minutes later.

## Wikilink resolution / rewriting

See [ADR-0002: Wikilink Sentinel Encoding](./adr/0002-wikilink-sentinel-encoding.md) first — this section is the runtime mechanics
that ADR describes.

**Parsing.** `wikiLinkRe` (`internal/links/helpers.go`) matches `[[Target]]` and
`[[Target|Alias]]`, capturing the target in group 1. It excludes `]`, `|`, `#` and `\n` from the
target and requires a non-space first character (so bash-style `[[ -n … ]]` doesn't match).
`extractWikiLinksFromMarkdown` (`helpers.go`) runs this regex over content, skipping
matches inside `MarkdownRefactorEngine.collectExcludedRanges` (code spans/blocks) and
de-duplicating targets.

**Link state.** Every stored link has a `LinkState` (`helpers.go`): `LinkResolved`,
`LinkBroken` or `LinkAmbiguous`. The state is computed on write (source page save, title
reconcile), never on read. `classifyTitleMatches` is the single rule for title links:
0 matches → broken, 1 → resolved, more than 1 → ambiguous. An ambiguous link is not broken: it
shows up as a backlink on every matching page (`mergeAmbiguousWikiLinksIntoBacklinks` in
`link_service.go`).

**Resolution.** `resolveWikiLinkTargets` (`internal/links/helpers.go`), called from
`collectTargetsFromContent` (`helpers.go`), for each target:
- **Title first, always.** `treeService.FindPagesByTitle` (exact `leafwiki_title` match) runs for
  every target, including ones containing `/`, so a page titled `TCP/IP` or `C/C++ Guide` wins
  over an unrelated page whose slug path happens to match.
- The result is stored under the sentinel `to_path = "wikilink:<Title>"` (`wikilinkSentinel()`),
  with the state from `classifyTitleMatches` and the target page ID when there is exactly one
  match. See [ADR-0002](./adr/0002-wikilink-sentinel-encoding.md) for why the sentinel is used
  regardless of state.
- **Path hint fallback.** Only when a target contains `/` and has **no** title match is it
  retried as a route path via `treeService.FindPageByRoutePath` (walks the tree matching slugs
  segment by segment). Found → stored as resolved with the real route path. Not found → stored
  as a broken link to `/<path>`, so `HealLinksForExactPath` heals it once a page is created at
  that path. An ambiguous title match is never retried as a path.
- The link index (`LinksStore`) always gets a row per wikilink, whatever its state.

**Keeping state current.** Two mechanisms, both driven by `pagesave.LinkIndexSideEffect.Apply`
(`internal/wiki/pagesave/link_effect.go`):
- `LinkService.ReconcileTitle(title)` recomputes the state of every `[[title]]` record against
  the current tree in a single `UPDATE` (all links to a title share one sentinel key, so the
  source pages aren't needed). Called for the page's title on create and restore, for old and
  new title when a title changes, and for every deleted page's title on delete.
- `HealLinksForExactPath` re-resolves path links that point at the page's route path; called
  after `update` without a slug change (`UpdateLinksForPage` then `healExact`) and from
  `updateAndHeal` on create/restore/move.
- On slug change and move, links to the old path (prefix) are marked broken and every affected
  page's outgoing links are re-indexed. On delete, outgoing links of the deleted pages are
  removed and incoming links are marked broken by page ID (which also catches resolved
  wikilink sentinels) and by old path.

Because this runs from page save, a change made outside the app (bulk import, direct
filesystem edit) isn't reflected until the next [Resync](#resync).

**Rewriting on rename/move** (opt-in, `EnableLinkRefactor` flag) is a separate mechanism from
healing — it actually edits markdown content in *other* pages, not just the index. See
[Tree rename/move](#tree-renamemove).

## Resync

See [ADR-0004: Resync Progress via Polling](./adr/0004-resync-progress-via-polling.md) first for why this is poll-based, not
push-based.

1. `POST /api/admin/resync` → `Routes.handleTriggerResync`
   (`internal/wiki/resync/routes.go`, admin-only) → `TriggerResyncUseCase.Execute`
   (`internal/wiki/resync/use_cases.go`): calls `job.Start()` on the shared `ResyncJob`
   (`internal/wiki/resync/job.go`, a mutex-guarded status struct); returns `409` via
   `ErrResyncAlreadyRunning` if a resync is already running, otherwise fires
   `w.launchReloadWithProgress` (a plain `go func()`, not tracked by the HTTP request) and
   responds `202` immediately.
2. `w.reloadWithProgress(ctx)` (`internal/wiki/wiki_resync.go`) wraps `runReloadPhases`, holding
   `w.reloadMu` for the **entire** operation (serializes resyncs; also recovers panics so a
   bug in one phase always calls `job.Finish(err)` rather than leaving the job stuck
   "running" forever). Four phases, each calling `job.SetPhase(...)`:
   - `PhaseTree` — `tree.ReconstructTreeFromFSContext(ctx)` (full re-walk of the markdown
     tree from disk; failure aborts the resync).
   - `PhaseLinks` — `links.IndexAllPagesContext(ctx)` (failure here is logged and continues,
     doesn't abort the whole resync).
   - `PhaseTags` — `w.bootstrapTagsAndProperties()`.
   - `PhaseSearch` — `pagesave.NewSearchIndexSideEffect(...).IndexAllPagesContext(ctx)`
     (failure here **does** abort with `finishErr` set — search is the one phase treated as
     load-bearing enough to fail the whole job).
3. `GET /api/admin/resync/status` → `GetResyncStatusUseCase.Execute` → `job.Status()`
   (`internal/wiki/resync/job.go`), returning `{running, phase, done, error}`.
4. Frontend (`ui/leafwiki-ui/src/stores/resync.ts`) polls this endpoint every
   **800ms** (`POLL_INTERVAL_MS`) until `done`.
5. `SIGUSR1`/`SIGHUP` (`cmd/leafwiki/signals_unix.go`; unavailable on Windows) also trigger a
   resync through the same `ResyncJob`, via `Wiki.TriggerResyncAsync`
   (`internal/wiki/wiki_resync.go`).

A context cancellation (shutdown) between or during phases ends the resync with
`ctx.Err()`.

**Known limitation:** boot-time indexing always does a full rebuild rather than trusting the
already-persisted SQLite indexes, because `search.SQLiteIndex`'s `ensureSchema()`
unconditionally drops the FTS5 `pages` table on every process start. Any fix for that must
leave the admin-triggered and signal-triggered resync flows described above untouched.

## Editor auto-save

Frontend-only; ties back into [Page save](#page-save) via the same `PUT /api/pages/:id`.

Two cooperating pieces: `usePageEditorStore.savePage`
(`ui/leafwiki-ui/src/features/editor/pageEditorStore.ts`) does the actual save (dirty
check, API call, local state sync); `useAutoSave`
(`ui/leafwiki-ui/src/features/editor/useAutoSave.ts`) is the hook that decides *when* to call
it unattended.

- **Trigger**: a `useEffect` watching `content`, `tags`, `frontmatterFields` (plus
  `autoSave`/`dirty`/`slug`/`status`/`retriggerCount` as re-arm conditions) sets a
  `setTimeout` debounce of **`DEBOUNCE_MS = 2000` (2 seconds)** (`useAutoSave.ts`)
  after every change — pure keystroke/edit debounce, not a fixed interval and not blur-based.
- Guard conditions checked both at schedule time and again when the timer fires (state may
  have changed during the 2s wait): auto-save must be on, status must not be `'paused'`
  (a version conflict pauses auto-save until a manual save succeeds), state must be dirty,
  **slug must be unchanged** (`page.slug !== slug` skips auto-save — slug changes require a
  manual save, because they can trigger the link-refactor confirmation flow, which isn't
  compatible with a silent background save), no save already in flight
  (`isSavingRef`), and frontmatter metadata must currently validate.
- A module-level `isSavingMutex` in `pageEditorStore.ts` prevents concurrent *auto*-saves
  from stacking; **manual saves (`silent: false`, e.g. Ctrl+S) bypass this mutex** so they're
  never blocked by an in-flight auto-save.
- `savePage` (`pageEditorStore.ts`): re-validates frontmatter, then — if the slug changed
  *and* `enableLinkRefactor` is on — runs the preview/confirm/apply refactor flow
  (`previewPageRefactor` → `confirmPageRefactor` dialog → `applyPageRefactor`) instead of a
  plain `updatePage` call (see [Tree rename/move](#tree-renamemove)); otherwise calls `updatePage` directly.
  On success it patches the local page snapshot, reloads the tree on structural changes
  (title/slug) or just patches the node version otherwise, syncs the viewer store if it's
  showing the same page, and refetches link/backlink status for the page.
- On `page_version_conflict` (stale optimistic-concurrency version — see `UpdateNode`'s
  `checkNodeVersion` in [Page save](#page-save)), auto-save sets status to `'paused'` and toasts;
  it un-pauses automatically once it detects `page.version` advancing (i.e. after a manual
  save succeeds) or once the editor becomes clean again.
- **Flush on unmount**: if a debounce timer was pending when the component unmounts
  (navigation away), the cleanup effect fires `savePage({silent: true})` immediately so
  edits made in the last <2s aren't lost, with a one-time 300ms-delayed retry if the save
  silently no-op'd because the mutex was held by a concurrent manual save.

## Tree rename/move

Two distinct mechanisms exist; which one is used depends on `EnableLinkRefactor`
(`cmd/leafwiki/main.go`, wired to `router.go`) and the frontend's `enableLinkRefactor`
config flag.

**Plain move** — `PUT /api/pages/:id/move` → `Routes.handleMove`
(`internal/wiki/pages/routes.go`) → `MovePageUseCase.Execute`
(`internal/wiki/pages/move_page.go`):
1. Collects the whole subtree's IDs via `collectSubtreeIDs(node)` before moving (position -1
   means append).
2. `tree.MoveNodeToPosition(...)` performs the actual move under the tree lock.
3. Re-fetches all subtree pages as `AffectedPages` and runs the standard
   `pagesave.PageOperationMove` event through the orchestrator (see [Page save](#page-save)) — this
   only marks old-path links broken and reindexes search/links/tags for the new paths; it
   does **not** rewrite `[[...]]` or markdown-link text inside *other* pages that reference
   the moved page by path.

**Rename** happens through the same `UpdatePageUseCase.Execute` as a content edit
(`internal/wiki/pages/update_page.go`) — when `Slug != before.Slug`, `UpdateNode`
recursively updates the affected subtree, and `pagesave.LinkIndexSideEffect` marks the old
path's links broken and (if the title also changed) marks incoming links broken by page ID
too — again, index maintenance only, no content rewriting elsewhere.

**Refactor (rewrite-links-on-rename/move)** — opt-in via
`POST /api/pages/:id/refactor/preview` and `/apply`
(`internal/wiki/pages/routes.go`, gated behind `opts.EnableLinkRefactor`):
- `PreviewPageRefactorUseCase.Execute` (`internal/wiki/pages/refactor.go`) computes
  `oldPath`/`newPath`, finds every other page whose content references the old path or (for a
  title-changing rename) the old title as a `[[Title]]` sentinel, and returns match counts +
  warnings without touching anything.
- `ApplyPageRefactorUseCase.Execute` (`internal/wiki/pages/refactor.go`):
  1. Rebuilds the plan and re-captures path-change snapshots.
  2. If `RewriteLinks`, calls `rewriteAffectedPages` (`refactor.go`) **first**: for every
     affected page, runs `MarkdownRefactorEngine.Rewrite` (markdown links) then
     `RewriteWikiLinksPrecompiled` (wikilinks) against a compiled `RewriteRule{OldPath,
     NewPath, OldTitle?, NewTitle?}`, batches changed content via
     `tree.BulkUpdateContent`, records content revisions
     (`revision.RecordContentUpdates`), and updates the link index
     (`links.UpdateRewrittenLinksAndHealForPages`).
  3. Then performs the actual move/rename: for `RefactorKindRename`, delegates to the same
     `UpdatePageUseCase`; for `RefactorKindMove`, delegates to the same `MovePageUseCase` —
     both wrapped in a **fresh, refactor-scoped** `pagesave.PageSaveOrchestrator` with only
     the link + revision side effects (`refactor.go`), not the full five-effect set.
  4. `rewritePathChangedSubtree` (`refactor.go`) then rewrites path-relative links
     *inside* the moved/renamed subtree's own pages (their own path changed, so relative
     links within them need updating even if no external page referenced them).
  5. If `RewriteLinks`, `refreshAffectedPageLinks` (`refactor.go`) does a final
     `UpdateLinksAndHealForPages` pass over affected pages.
- The frontend triggers this automatically from `pageEditorStore.savePage` whenever a manual
  save changes the slug and `enableLinkRefactor` is on — see [Editor auto-save](#editor-auto-save) — via a
  preview → user-confirmation dialog (`confirmPageRefactor`, `allowSkipRewrite: true`) →
  apply sequence, not silently.

**Recursive delete** (`DELETE /api/pages/:id?recursive=true`,
`internal/wiki/pages/delete_page.go`) follows the same subtree-collection pattern:
`collectSubtreeIDs` before deleting, a single `pagesave.PageOperationDelete` event over the
whole `AffectedPages` list, then per-page asset/favorite cleanup and
`deleteRevisionData` for every subtree ID.

## Auth

Full auth *model* (JWT structure, cookie strategy, role model) is covered in
[LeafWiki Architecture](./architecture.md); this section is the request-by-request flow only.

**Native login** — `POST /api/auth/login` → `Routes.handleLogin`
(`internal/wiki/auth/routes.go`) → `LoginUseCase.Execute`
(`internal/wiki/auth/use_cases.go`) → `AuthService.Login`
(`internal/core/auth/auth_service.go`):
1. Looks up the user by identifier; on failure runs a dummy `bcrypt.CompareHashAndPassword`
   anyway (constant-time-ish defense against username enumeration via timing) before
   returning `ErrUserInvalidCredentials`.
2. `a.attempts.recordAttempt(user.ID)` — per-user lockout counter; returns
   `ErrUserAccountLocked` if tripped.
3. Verifies the password hash. On success, if `user.TOTPEnabled`: does **not** reset the
   attempt counter (TOTP brute-forcing shares the same counter as password brute-forcing —
   see the comment at `auth_service.go`) and returns a **TOTP challenge** instead of a
   session (`beginTOTPChallenge`) — the HTTP layer responds `{requiresTotp: true,
   loginChallengeToken}` with **no cookies set** (`routes.go`).
4. If TOTP isn't enabled: resets the attempt counter and calls `sessions.IssueSession(user)`
   directly, issuing access + refresh tokens.
5. On a non-TOTP success, the handler issues the CSRF cookie (`CSRFCookie.Issue`) then sets
   auth cookies (`AuthCookies.Set`) before responding `{user, accessTokenExpiresAt}`.

**TOTP second step** (only when step 3 above returned a challenge) — `POST
/api/auth/login/totp` → `handleLoginTOTP` (`routes.go`) → `CompleteTOTPLoginUseCase` →
`AuthService.CompleteTOTPLogin` (`auth_service.go`): validates the single-use, short-lived
`loginChallengeToken` (session-store lookup by `jti`), records another lockout attempt against
the *same* per-user counter as the password step, verifies the code as either a live TOTP code
or an unused recovery code, resets the counter and revokes the challenge token on success, then
issues the session exactly as a non-TOTP login would. Only at this point are cookies set.

**Token refresh** — `POST /api/auth/refresh-token` → `RefreshTokenUseCase.Execute` →
`AuthService.RefreshToken` → `sessions.RefreshToken`. Frontend drives this from
`ui/leafwiki-ui/src/lib/api/auth.ts`:
- Proactively, before any authenticated request: `shouldRefreshBeforeRequest` refreshes if
  the access token expires within **`REFRESH_THRESHOLD_SECONDS = 60`** seconds
  (`auth.ts`).
- Reactively, on a `401` response: retries the request once after a refresh
  (`auth.ts`).
- Both paths funnel through `ensureRefresh()`, which dedupes concurrent refresh calls via a
  single shared `globalThis.__leafwikiRefreshPromise` (`auth.ts`) so simultaneous
  in-flight requests don't each trigger their own refresh.
- Server-side default lifetimes: access token **15m**, refresh token **168h / 7 days**
  (`--access-token-timeout` / `--refresh-token-timeout`, `cmd/leafwiki/flags_auth.go`).
- Both `authDisabled` and reverse-proxy remote-user mode (below) skip all of this — there's no
  token to refresh in either mode.

**Reverse-proxy header-trust path** skips native login entirely.
`InjectRemoteUser` (`internal/http/middleware/auth/reverse_proxy.go`) runs ahead of
`RequireAuth` on every request: if enabled and the request's remote address is in the
configured trusted-proxy list, it reads a configured header (e.g. `X-Remote-User`), resolves
it to a user via `UserService.GetUserByIdentifier`, and — if found — stores that user directly
in the Gin context (`c.Set("user", user)`). `RequireAuth`
(`internal/http/middleware/auth/auth.go`) checks for that pre-set context user **first**
and short-circuits straight past JWT/cookie validation if present. If the trusted proxy sends
an unrecognized identifier, the request is rejected with `401` rather than falling through to
normal auth (a misconfigured proxy must not silently downgrade to anonymous). No
`POST /api/auth/login` call ever happens in this mode; there is no access/refresh token pair
to manage on the frontend either.
