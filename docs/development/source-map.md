# LeafWiki Source Map

Package-by-package tour of the `wiki` repo (Go backend under `internal/`, `cmd/`; React SPA
under `ui/leafwiki-ui/src/`). Companion to [LeafWiki Architecture](./architecture.md) (layering/storage/auth model)
and [LeafWiki Processes](./processes.md) (end-to-end workflow walkthroughs) — this page answers "what lives
where," those answer "how does layer X work" and "how does flow Y run." Part of [Developer Docs](./README.md).


## Backend layering (one-paragraph orientation)

`cmd/leafwiki` (entrypoint) constructs an `internal/wiki.Wiki` (composition root), which wires
`internal/core/*` (filesystem/DB primitives — tree, auth, markdown, revision) into the top-level
derived-index packages (`internal/links`, `internal/tags`, `internal/properties`, `internal/search`,
`internal/favorites`, `internal/usersettings`, `internal/avatar`, `internal/publicaccess`,
`internal/tocdisplay`, `internal/snapshot`, `internal/restore`, `internal/backup`,
`internal/branding`, `internal/importer`), then into the `internal/wiki/<domain>` slices (`pages`,
`pagesave`, `auth`, `resync`, `revisions`, `search`, `links`, `tags`, `properties`, `apikeys`,
`assets`, `avatar`, `backup`, `branding`, `health`, `importer`, `instancesettings`, `restore`,
`snapshot`, `usersettings`) that each expose HTTP routes via
`internal/http`. See [ADR-0001: Filesystem as Source of Truth](./adr/0001-filesystem-source-of-truth.md) for why the split between
filesystem-canonical (`tree`, `revision`) and SQLite-derived (`links`, `tags`, `properties`,
`search`) packages exists.

## `cmd/leafwiki`

- **`cmd/leafwiki/cli.go`** — the `urfave/cli/v3` command tree: the server command plus operator
  subcommands (`reset-admin-password`, offline snapshot restore).
- **`cmd/leafwiki/flags_*.go`** — flag and env var definitions, one file per area (`server`,
  `auth`, `backup`, `email`, `frontend`, `metrics`, `proxy`). `leafwiki --help` is generated
  from these; `docs/` documents them.
- **`cmd/leafwiki/main.go`** — `runServerCommand`: `slog` logger setup, builds `wiki.WikiOptions`
  and the services that depend on flags (e.g. `buildPublicAccessService`), constructs
  `wiki.Wiki`, the `internal/http` router, Prometheus metrics and the backup/snapshot schedulers;
  owns graceful shutdown (context cancellation, `net/http.Server.Shutdown`).
- **`cmd/leafwiki/signals_unix.go`** / **`signals_windows.go`** — OS-specific signal wiring, split
  via Go build tags (`//go:build !windows` / `//go:build windows`). Unix listens for `SIGUSR1` and
  `SIGHUP` to trigger a live reload; Windows has no equivalent (`os/signal` doesn't support those
  signals there), so `notifyReloadSignals` is a no-op — a concrete example of the cross-platform
  seam called out in `AGENTS.md` (Linux/Windows/Raspberry Pi single binary).
- **`cmd/leafwiki/main_test.go`** — tests around flag/startup wiring.

## `internal/core/*` — filesystem/DB primitives (no HTTP, no use-case orchestration)

- **`internal/core/tree`** — the canonical in-memory page tree plus its on-disk representation
  (markdown files + frontmatter under `root/`). `TreeService` (guarded by a single `sync.RWMutex`)
  is the one component allowed to mutate the tree: create/move/delete/copy pages and sections.
  `tree_service.go` holds the type, its lifecycle and the lock helpers; mutations live in
  `tree_mutations.go`, reads in `tree_reads.go`, the lookup indexes in `tree_index.go`.
  `node_store.go` holds the `NodeStore` type, page reads and the path/metadata helpers;
  `node_store_reconstruct.go` rebuilds the tree from disk (walks `root/`, reads `leafwiki_id`
  frontmatter); `node_store_write.go` holds the file write paths.
  `schema.go` tracks an on-disk schema version (`CurrentSchemaVersion`,
  currently 5) used to gate one-time migrations. `slug_service.go` handles slug generation/collision
  suggestions. `migration_adapter.go` bridges to `internal/core/treemigration` for legacy-format
  upgrades.
- **`internal/core/treemigration`** — standalone migration runner (`runner.go`) that walks an
  older on-disk tree format and adds missing frontmatter/IDs; kept separate from `tree` so the
  migration logic doesn't get entangled with steady-state tree operations.
- **`internal/core/markdown`** — markdown-file abstraction independent of the tree: `markdown.go`
  (`MarkdownFile` — path-aware read/write with title extraction), `frontmatter.go` (YAML frontmatter
  parse/serialize), `reserved.go` (`IsSystemKey` — `tags` and any `leafwiki_`-prefixed key are
  reserved, `title` is not and round-trips as a normal custom property).
- **`internal/core/auth`** — local identity and session primitives, largest `core` package.
  `auth_service.go` (`AuthService`, password+TOTP login orchestration, `loginChallengeLifetime` =
  5 min bridging the two login steps), `session_manager.go` (`SessionManager` — JWT access/refresh
  issuance, refresh, revoke; deliberately decoupled from *how* a user was authenticated, so a
  future OIDC handler could reuse it), `session_store.go` (SQLite `sessions.db`), `user_service.go`
  / `user_store.go` (`users.db`), `user_resolver.go` (`UserResolver` — ID→username/email lookup
  cache used by e.g. revision authorship display; negative/not-found lookups are cached too),
  `totp_service.go` (AES-256-GCM-encrypted-at-rest TOTP secrets, `pquerna/otp`), `apikey_service.go`
  / `apikey_store.go` (API key auth), `login_attempt_tracker.go` (brute-force throttling),
  `email_token_store.go` / `email_token_service.go` (SQLite `email_tokens.db` — single-use
  password-reset/invite tokens, compare-and-swap consume idiom copied from
  `UserStore.ConsumeRecoveryCodeHash`; composes `internal/core/email.Service` for delivery and
  `AuthService` for post-reset session revocation / invite auto-login — see
  [ADR-0011: SMTP as a CLI/ENV-Only Optional Feature](./adr/0011-smtp-cli-env-optional-feature.md)).
- **`internal/core/revision`** — content-addressed revision history: `fs_store.go` writes
  SHA-256-addressed content blobs plus JSON metadata to disk, `service.go` (`RevisionService`)
  is the orchestration layer used by `internal/wiki/revisions` and `pagesave`'s revision effect.
  See [ADR-0003: Revision Coalescing (Fixed Window)](./adr/0003-revision-coalescing-fixed-window.md) for the fixed-window auto-save coalescing
  policy implemented on top of this.
- **`internal/core/assets`** — per-page file attachments (`assets_service.go`): upload, list,
  delete, path resolution under each page's asset directory; enforces the configured max upload
  size.
- **`internal/core/excerpt`** — `excerpt.go` renders a short plain-text excerpt (`MaxRunes = 180`)
  from a page's markdown via `goldmark` + `bluemonday` HTML sanitization, used by search results
  and tag listings.
- **`internal/core/ignore`** — `.leafwikiignore` support (`gitignore`-syntax matcher via
  `sabhiram/go-gitignore`), consulted by tree reconstruction/resync/backup to skip paths.
- **`internal/core/shared`** — cross-cutting helpers with no business logic of their own:
  `utils.go` (`GenerateUniqueID` via `teris-io/shortid`, misc path/error helpers),
  `shared/errors/localized_error.go` (`*LocalizedError` — the required domain-error type for all
  backend domain errors per `AGENTS.md`, with `internal/backup` as the one documented legacy
  exception), `shared/htmlutil/html.go` (HTML
  escaping helpers), `shared/sqliteutil/sqlite.go` (`OpenWithPolicy` + `CorruptionPolicy` —
  each SQLite store names at its constructor what happens on a recoverable I/O/corruption error,
  `RebuildOnCorruption` for derived indexes, `QuarantineOnCorruption` / `FailOnCorruption`
  (typed `*CorruptDatabaseError`) for real state, see
  [ADR-0013](./adr/0013-per-store-sqlite-corruption-policy.md); `IsSQLiteRecoverableError`
  distinguishes those errors from transient `SQLITE_BUSY`/`SQLITE_LOCKED`).
- **`internal/core/tools`** — small operator CLI helpers, e.g. `adminreset.go`
  (`ResetAdminPassword`, behind the `leafwiki reset-admin-password` subcommand in `cli.go`).
- **`internal/core/settings`** — shared persistence primitive for the small JSON settings files
  in the data directory (`branding.json`, `public-access.json`, …). `value.go` defines
  `Value[T]` with two implementations: `Managed[T]` (changeable at runtime, persisted) and
  `Fixed[T]` (pinned by a flag/env var; `Put` returns `ErrEnvManaged`). The SQLite stores are not
  covered by this.
  Deliberately separate from the SMTP token machinery below.
- **`internal/core/email`** — optional SMTP integration (`--smtp-host`/etc., CLI/ENV-only —
  [ADR-0011: SMTP as a CLI/ENV-Only Optional Feature](./adr/0011-smtp-cli-env-optional-feature.md)): `config.go` (`Config.Enabled()` =
  `Host != ""`), `client.go` (stdlib `net/smtp` wrapper; STARTTLS/plaintext dial `net.Dialer`,
  implicit TLS dials `tls.Dial` manually since stdlib has no helper for that mode), `templates.go`
  (`html/template`/`text/template`, link-only interpolation — no username/site-name fields, so
  there's nothing to injection-guard beyond the auto-escaping template engines already do),
  `service.go` (`Service.Send{PasswordReset,Invite}Email`, consumed by `internal/core/auth`'s
  `EmailTokenService`).

## Top-level derived-index and infrastructure packages (`internal/*`, non-`core`, non-`wiki`)

These sit between `core` and the `wiki/<domain>` HTTP slices. Per [ADR-0001](./adr/0001-filesystem-source-of-truth.md), `links`/`tags`/`properties`/`search` are SQLite caches fully rebuildable from the tree;
`favorites`/`usersettings`/`avatar`/`snapshot`/`restore`/`backup`/`branding` are not
resync-derived state.

- **`internal/links`** — wikilink graph index. `links_store.go` (SQLite `links.db`),
  `link_service.go` (`LinkService`), `backlink.go`/`outgoing.go` (incoming/outgoing link queries),
  `link_refactor.go` (`MarkdownRefactorEngine`, goldmark-AST-based link-text rewriting used when a
  page is renamed/moved). Each stored link carries a `LinkState` (resolved/broken/ambiguous), see
  [Processes: wikilink resolution](./processes.md#wikilink-resolution--rewriting). See
  [ADR-0002: Wikilink Sentinel Encoding](./adr/0002-wikilink-sentinel-encoding.md) for
  how `[[Title]]` links are encoded internally.
- **`internal/tags`** — tag index: `tags_store.go` (`tags.db`), `tags_service.go`
  (`TagsService.ClearIndex` + lookup/filter by tag), `excerpt.go` (tag-scoped excerpt rendering).
- **`internal/properties`** — arbitrary-frontmatter-property index (custom, non-reserved
  frontmatter keys): `properties_store.go` (`properties.db`), `properties_service.go`.
- **`internal/search`** — full-text search index: `sqlite_index.go` (`search.db`, FTS via
  `modernc.org/sqlite`, goldmark-based content extraction + `htmlutil` sanitization),
  `indexing_status.go`, `search_result.go`.
- **`internal/usersettings`** — per-user preferences (language, auto-save, date/time format):
  `usersettings_store.go` (`usersettings.db`), `usersettings_service.go`, `language.go`
  (allowed languages), `dateformat.go`. Real state, part of full backup/restore.
- **`internal/avatar`** — per-user avatar upload/delete (`AvatarService`, allowed image
  extensions, stored as files in the data directory).
- **`internal/publicaccess`** — the public-access ("public mode") switch. `NewEnvManaged` when
  pinned by `--public-access`/`--disable-auth`, `NewSettingsManaged` when changeable at runtime
  via `internal/core/settings`.
- **`internal/scopedread`** — the read boundary for callers that may see only part of the wiki
  (share links, #1252): `ReadScope` (a root page, optionally with its subtree) and
  `PageReader`, which checks the scope before reading and returns an allow-listed `PageView`.
  Out of scope, unknown and deleted pages are the same `ErrNotFound`.
  [ADR-0015](./adr/0015-scoped-read-boundary.md).
- **`internal/sharing`** — share links (#1252, experimental, `--enable-share-links`):
  `share_link_store.go` (`share_links.db`, own file, real state, part of full backup/restore;
  only a SHA-256 hash of each token is stored), `share_link_service.go` (create, revoke,
  resolve a token to a `Grant`; unknown, malformed, expired and revoked tokens all give
  `share_link_not_found`), `token.go` (256-bit `lws_` tokens). What a resolved link may read is
  decided by the scoped read layer (`internal/scopedread`, ADR-0015).
- **`internal/tocdisplay`** — the instance-wide "always show table of contents" flag, same
  provider pattern (`Provider` interface, `Fixed` for flag-pinned values).
- **`internal/favorites`** — per-user private favorited-pages list. `favorites_store.go`
  (`favorites.db`, own file). Explicitly documented in its own package doc comment as *not*
  filesystem-derived and therefore never touched by resync.
- **`internal/snapshot`** — periodic/manual full-instance snapshots (distinct from page revisions).
  `manager.go` (`Manager`, `ErrAlreadyRunning` guard), `scheduler.go` (`Scheduler` — ticker-based,
  `interval == 0` means manual-only mode with a `minSnapshotInterval` of 1 minute), `snapshot.go`,
  `status.go`.
- **`internal/restore`** — restoring an instance from a snapshot/backup ZIP: `manager.go`
  (`ErrRestoreAlreadyRunning`), `job.go` (phases: `validating` → `swapping` →
  `reopening_auth` → `invalidating_sessions` → `reloading_branding`), `swap.go` (hot-swap of
  on-disk state), `writegate.go` (blocks mutating requests during a restore — paired with
  `internal/http/middleware/maintenance`), `selfrestart_unix.go` / `selfrestart_windows.go`
  (OS-specific self-restart after restore, same unix/windows split pattern as `cmd/leafwiki`),
  `offline.go` (CLI-driven restore path outside the running server). See
  [ADR-0009: Session Manager Extraction](./adr/0009-session-manager-extraction.md) for why session invalidation is its own restore phase.
  State databases other than `users.db` are registered once in `stateDBs` (`swap.go`): snapshots
  capture them (`StateDBFiles`), a restore sanity-checks and swaps them, and the composition root
  passes the live stores as `Config.Stores` (`Wiki.StateStores`).
  [ADR-0016](./adr/0016-state-store-registration.md).
- **`internal/backup`** — git-based off-site backup of the markdown tree (`repo.go`, `go-git/go-git`
  — no shell-out to a `git` binary, matters for the single-binary story), `scheduler.go` (same
  manual-mode-at-interval-0 pattern as `snapshot`), `conflict_test.go`/`gitignore.go`,
  `status.go`. A pull that cannot proceed sets `NeedsIntervention` with a `ConflictKind`:
  `diverged` (no fast-forward either way; `ForcePush` overwrites the remote) or `fileConflict`
  (a remote change hits a locally changed file; nothing is applied, local stays behind the
  remote, and `ForcePush` refuses with `ErrForcePushRemoteAhead` → 409
  `backup_force_push_conflict`, because overwriting would drop the remote commits).
  `compareWithRemote` decides the relation for both the pull and `ForcePush`; `ForcePush` leases
  its push on the remote tip it compared, so a commit pushed in between is not overwritten. Don't
  move the backup branch ref by hand to clear a conflict: a local branch behind the remote is
  exactly the state `ForcePush` refuses. `redact.go` masks remote credentials in transport
  errors before they reach logs or the status. Documented exception to the `*sharederrors.LocalizedError` convention (uses plain
  `errors`/`fmt.Errorf`) — see `AGENTS.md`.
- **`internal/branding`** — custom logo/favicon/site-name: `branding_service.go`,
  `branding_store.go`, `models.go`, `utils.go`.
- **`internal/importer`** — bulk import of external content packages (e.g. Obsidian vaults,
  nested LeafWiki exports) into the tree: `planner.go` (`Planner` — dry-run diff of what an import
  would do), `executor.go` (`ExecutionResult`, applies a plan), `content_transformer.go`
  (per-format content rewriting), `zip_extractor.go`/`zip_workspace.go`, `plan_store.go`,
  `importer_service.go`. Fixtures for each supported source format live under
  `internal/importer/fixtures/`.
- **`internal/http`** — the shared router *infrastructure* (not a domain slice itself).
  `router.go` builds the `gin.Engine`, embeds the built frontend (`//go:embed dist/**`, gated by
  the `EmbedFrontend` build-time flag), sets up request logging via `slog`. `shell.go` serves
  the SPA fallback route: it runs the registered `ShellResponder`s and renders `index.html`
  from the `ShellContributor`s on `FrontendConfig` (title, head, `#root` markup; built-ins for
  the custom stylesheet and `InjectCodeInHeader` last), see
  [ADR-0017](./adr/0017-spa-shell-contributors.md). `registrar.go` defines
  the `RouteRegistrar` interface and `RouterContext` (engine, auth cookie manager, CSRF cookie
  manager, options; `APIAuthGroup`, `APIReadGroup`, and `CapabilityGroup` for identity-free
  capability URLs, [ADR-0018](./adr/0018-capability-routes-without-identity.md)) that every
  `internal/wiki/<domain>` package's `Routes` implements/receives —
  this is the seam that lets each domain slice register its own routes/middleware without the
  central router knowing about it. `dto/` holds shared HTTP response types
  (`dto/page.go`, `dto/tag.go`, `dto/property.go` — explicitly forbidden from importing
  `internal/wiki` to avoid import cycles). `metrics/metrics.go` is the Prometheus registry
  (`HTTPMetrics`) used by the load tests in `loadtest/`;
  `metrics/runtime_collector.go` adds a pull-based scrape-time collector for
  `leafwiki_pages{kind}` / `leafwiki_users{role}` / `leafwiki_users_with_totp` /
  `leafwiki_users_pending_invite`, fed by `TreeService.NodeCounts()` and the
  `UserService` via the `internal/wiki/metrics_runtime.go` adapter.
  - **`internal/http/middleware/auth`** — `auth.go` (`RequireAuth`), `auth_cookie.go`
    (access/refresh cookie read/write), `apikey.go` (`InjectAPIKeyUser`, Bearer-token auth),
    `reverse_proxy.go` (`InjectRemoteUser` — trusted-header auth, see
    [ADR-0005: Reverse-Proxy Header Auth](./adr/0005-reverse-proxy-header-auth.md) and [ADR-0010: Remote User Auto-Provisioning](./adr/0010-remote-user-auto-provisioning.md)),
    `trusted_proxies.go`, `current_user.go` (`MustGetUser`/`TryGetUser` accessors used throughout
    the `wiki/<domain>` route handlers), `inject_public_editor.go` and `require_auth_or_public.go`
    (public-access mode support).
  - **`internal/http/middleware/security`** — `csrf.go`/`csrf_cookie.go` (double-submit CSRF,
    only enforced on mutating methods), `rate_limiter.go` (`KeyedLimiter` — sliding-window,
    reusable both as a full middleware and inline inside another middleware, e.g. API-key auth),
    `untrusted_file.go` (response headers for serving user-uploaded files).
  - **`internal/http/middleware/maintenance`** — `writegate.go`: blocks mutating requests during
    a restore, with an explicit segment-based (not substring-based) exemption list so the restore
    endpoint itself and a few other paths stay reachable while the gate is up.
  - **`internal/http/middleware/utils`** — `require_secure.go`, small shared middleware helper.
- **`internal/test_utils`** — `common.go`, shared test scaffolding (e.g. temp-dir wiki setup)
  used across package tests.

## `internal/wiki` — composition root and per-domain HTTP slices

- **`internal/wiki/wiki.go`**, **`wiki_boot.go`**, **`wiki_routes.go`**, **`wiki_resync.go`** — the composition root, split by concern: `wiki.go` holds the `Wiki` struct, `NewWiki(options)` and `Close()`; `wiki_boot.go` the `init*` steps, welcome page and baseline revisions; `wiki_routes.go` `buildRoutes`, the `build<Name>Routes` helpers and `newPageOrchestrator`; `wiki_resync.go` `reloadWithProgress`/`runReloadPhases` and `TriggerResyncAsync`. `NewWiki` wires every
  `core`/top-level package above into a single object, in a fixed init order (`initAuth` →
  `initEmail` → `initCoreServices` → `initLinkService` → `initTagsService` →
  `initPropertiesService` → `initFavoritesService` → `initUserSettingsService` →
  `bootstrapTagsAndProperties` → `initSearch` → `initBranding` → `initAvatarService` →
  `EnsureWelcomePage` → baseline revisions → `buildRoutes`). This is the literal "composition root" referenced in `AGENTS.md`'s
  architecture summary. `wiki.go` (`NewWiki`) is the best single entry point for tracing how
  a request-time dependency gets built.
- **`internal/wiki/metrics_runtime.go`** — adapter feeding tree and user counts into the
  Prometheus runtime collector.
- **`internal/wiki/import_adapter.go`** — `WikiImportAdapter`, implements the importer package's
  narrow `ImporterWiki` interface using `Wiki`'s already-constructed services, so `internal/importer`
  doesn't need to depend on the `wiki` package's full surface.
- **`internal/wiki/errors.go`** — shared error-response helpers used by multiple domain slices.

Every subpackage below follows the same three-file shape (visible in e.g.
`internal/wiki/tags/{routes,use_cases,errors}.go`): **`routes.go`** defines a `Routes` struct
implementing `internal/http`'s `RouteRegistrar`, built from a `RoutesConfig` of use cases plus
`AuthService`; **`use_cases.go`** holds one `XxxUseCase` type per operation (`Execute(ctx, input)`),
the application layer translating HTTP-shaped input into calls on `core`/top-level services and
`http/dto` types; **`errors.go`** maps `*sharederrors.LocalizedError` codes to HTTP status/JSON
shape for that domain.

- **`internal/wiki/pages`** — the largest domain slice: CRUD + tree operations exposed over HTTP.
  One file per use case — `create_page.go`, `update_page.go`, `move_page.go`, `copy_page.go`,
  `delete_page.go`, `convert_page.go` (page↔section), `get_page_view.go` (`GetPageViewByIDUseCase`/`GetPageViewByPathUseCase` — the single-page view shared by API and SEO: page plus frontmatter tags and properties via `page_metadata.go`; not a visibility layer), `get_page.go` (`PageQueries` — the remaining read-only lookups: path lookup, title, permalink), `pin_page.go` (global editorial pin, see
  distinction from favorites below), `ensure_path.go`, `refactor.go` (link-refactor
  trigger), `add_favorite.go`/`remove_favorite.go`/`list_favorites.go` (per-user favorites, note the explicit distinction: pin is global/editorial via
  `tree.SetPinned` in page frontmatter, favorites is per-user via `internal/favorites`, deliberately
  *not* threaded into the shared `Node`/tree DTO since the in-memory tree is shared across all
  users' requests).
- **`internal/wiki/pagesave`** — the page-mutation side-effect pipeline, not itself a route
  package (no `routes.go`). `orchestrator.go` (`PageSaveOrchestrator.Run` — fans a `PageSaveEvent`
  out to every registered `PageSideEffect`, synchronously, metrics-timed per effect, best-effort:
  errors are handled inside each effect, never abort the chain — this is the concrete mechanism
  behind [ADR-0001: Filesystem as Source of Truth](./adr/0001-filesystem-source-of-truth.md)'s "index updates are best-effort, recoverable
  via resync" consequence). `event.go` (`PageSaveEvent` — operation type, before/after page state,
  changed-flags, affected pages). Individual effects: `link_effect.go`, `tags_effect.go`,
  `properties_effect.go`, `search_effect.go`, `revision_effect.go`.
- **`internal/wiki/resync`** — rebuild-derived-indexes-from-filesystem: `job.go` (`ResyncJob`,
  phases `tree`→`links`→`tags`→`search`, safe for concurrent `Status()` reads), `use_cases.go`
  (`TriggerResyncUseCase`, `GetResyncStatusUseCase`). Progress is polled, not pushed — see
  [ADR-0004: Resync Progress via Polling](./adr/0004-resync-progress-via-polling.md).
- **`internal/wiki/revisions`** — page revision history HTTP surface over
  `internal/core/revision`; `use_cases.go` for list/get/restore-a-revision.
- **`internal/wiki/search`**, **`internal/wiki/tags`**, **`internal/wiki/properties`**,
  **`internal/wiki/links`** — thin HTTP slices over the matching top-level `internal/<name>`
  package (search query endpoint, tag listing/filter, property listing/filter, backlink/link-status
  endpoints respectively).
- **`internal/wiki/apikeys`** — API key CRUD HTTP surface over `core/auth`'s `APIKeyService`.
- **`internal/wiki/assets`** — per-page asset upload/list/delete HTTP surface over
  `core/assets`.
- **`internal/wiki/auth`** — login/logout/refresh/TOTP-setup HTTP surface over `core/auth`'s
  `AuthService`/`SessionManager`/`TOTPService`.
- **`internal/wiki/backup`**, **`internal/wiki/snapshot`**, **`internal/wiki/restore`** — trigger/
  status HTTP surfaces over the matching top-level `internal/{backup,snapshot,restore}` packages;
  `restore/routes_test.go` and `snapshot/routes_test.go` cover the admin-triggered flows.
- **`internal/wiki/branding`** — branding settings HTTP surface (logo/favicon upload, site name).
- **`internal/wiki/health`** — liveness/readiness endpoint(s), no use-case-per-file split (small
  enough to stay in `use_cases.go`/`routes.go` directly).
- **`internal/wiki/importer`** — import-plan create/execute/cancel HTTP surface over
  `internal/importer`.
- **`internal/wiki/avatar`**, **`internal/wiki/usersettings`** — HTTP surfaces over
  `internal/avatar` and `internal/usersettings` (own avatar, own preferences).
- **`internal/wiki/instancesettings`** — admin-only instance settings changeable at runtime
  instead of via flag + restart; today the public-access toggle.

## Frontend — `ui/leafwiki-ui/src/`

React SPA, Zustand for state (one feature-local store per domain, see
[ADR-0006: Zustand Feature-Local Stores](./adr/0006-zustand-feature-local-stores.md)), `react-i18next` for all UI strings, shadcn/Radix
primitives for form controls. Entry: `main.tsx` → `App.tsx` (also owns calling
`loadFavorites`/`clearFavorites` as session state changes, `useApplyDesignMode.tsx` for
light/dark mode application).

### `features/*` — one folder per UI-facing domain

- **`features/apikeys`** — API key management screen + create/delete dialogs
  (`ApiKeysManagement.tsx`, `ApiKeyFormDialog.tsx`, `DeleteApiKeyDialog.tsx`), toolbar action
  registration (`useToolbarActions.tsx`).
- **`features/assets`** — per-page asset manager dialog and grid (`AssetManagerDialog.tsx`,
  `AssetManager.tsx`, `AssetItem.tsx`, hover preview `AssetPreviewTooltip.tsx`).
- **`features/auth`** — login form (`LoginForm.tsx`), route guards (`RequireAuth.tsx`,
  `AuthRedirect.test.tsx`), external-IdP redirect handling (`ExternalRedirect.tsx`).
- **`features/backup`** — git backup settings panel and its warning indicator when backup needs
  manual intervention (conflict) — `BackupSettings.tsx` (Force Push only for the `diverged`
  conflict kind, an explanation for `fileConflict`), `BackupWarningIndicator.tsx`.
- **`features/branding`** — branding settings form (`BrandingSettings.tsx`).
- **`features/designtoggle`** — light/dark mode toggle (`DesignToggle.tsx`, `designmode.ts`).
- **`features/editor`** — the markdown editor, the single biggest feature folder:
  `MarkdownEditor.tsx`/`MarkdownCodeEditor.tsx` (CodeMirror-based; `MarkdownEditor` composes
  `useEditorFileUpload.ts` for paste/drop uploads, `useSplitPaneResize.ts` for the persisted
  editor/preview split and `usePreviewScrollSync.ts` for cursor-to-preview scrolling),
  `MarkdownToolbar.tsx`,
  `pageEditorStore.ts` (feature-local Zustand store owning draft content/dirty state),
  `useAutoSave.ts` (drives the fixed-window revision coalescing from
  [ADR-0003: Revision Coalescing (Fixed Window)](./adr/0003-revision-coalescing-fixed-window.md)), `useNavigationGuard.tsx`/
  `useExternalUnloadBlocker.ts` (unsaved-changes protection), `internalLinkCompletion.ts`
  (`[[` wikilink autocomplete), `frontmatter.ts`, `pasteImageUpload.ts`, `htmlToMarkdown.ts`
  (paste-from-rich-text conversion), `PageFrontmatterPanel.tsx`.
- **`features/favorites`** — per-user favorite star toggle and sidebar list
  (`FavoriteToggleButton.tsx`, `FavoritesSection.tsx`, `FavoriteItem.tsx`).
- **`features/history`** — page revision history viewer + restore dialog
  (`PageHistoryContent.tsx`, `RevisionDetailPanels.tsx` for the changes/diff, preview,
  raw-text and assets panels, `lineDiff.ts` for the line diff, `RestoreRevisionDialog.tsx`,
  `pageHistory.ts`).
- **`features/imagepreview`** — click-to-zoom image preview dialog for content images.
- **`features/importer`** — bulk-import UI (`Importer.tsx`, view-state derivation in
  `importerState.ts`) driving `internal/wiki/importer`'s
  plan create/execute flow.
- **`features/links`** — backlink/outgoing-link status panel (`LinkInfo.tsx`,
  `linkStatusStore.ts`).
- **`features/page`** — page-level dialogs and utilities not owned by the editor/viewer:
  create/copy/move/delete dialogs, `SortPagesDialog.tsx`, `PageRefactorDialog.tsx` (bulk
  link-refactor trigger UI), `SlugInputWithSuggestion.tsx`, `PermalinkDialog.tsx`/
  `PermalinkRedirect.tsx`, `RootRedirect.tsx`.
- **`features/page-switcher`** — Cmd/Ctrl-K quick page switcher
  (`PageQuickSwitcherDialog.tsx`, `pageQuickSwitcher.ts`).
- **`features/preview`** — markdown-to-HTML rendering pipeline used by both the editor's live
  preview and the read-only viewer: `MarkdownPreview.tsx` (sanitize schema in
  `previewSanitizeSchema.ts`, error boundary in `MarkdownPreviewErrorBoundary.tsx`, pure
  renderer helpers in `previewHelpers.ts`), custom renderers
  (`MarkdownCodeBlock.tsx`, `MarkdownImage.tsx`, `MarkdownLink.tsx`, `MermaidBlock.tsx`,
  `Headline.tsx`, `MarkdownPdfEmbed.tsx`, `MarkdownInlineCode.tsx`), rehype/remark plugins
  (`rehypeLineNumber.ts`, `rehypeCodeFenceLineNumbers.ts`, `rehypeWhitelistStyles.ts`,
  `remarkImageSize.ts`), markdown normalizers (`normalizeMarkdownBlocks.ts`,
  `normalizeMarkdownListIndentation.ts`), syntax highlighting (`syntaxHighlightLanguages.ts`,
  `splitHighlightedLines.ts`), and the table of contents (`extractTocEntries.ts`,
  `TocSidePanel.tsx`, `TocDropdownButton.tsx`, `useTocScrollSpy.ts`, `tocVisibility.ts`).
  Shared Markdown typography lives in `src/index.css`: screen prose is limited to 680px,
  with 16px text on phones and 17px from the `sm` breakpoint. Media, code and table
  blocks retain the available surface width in editor/history previews. The viewer centers
  all content in one 680px reading column, including media, code and tables, aligned with
  breadcrumbs, metadata and backlinks. Wrapped breadcrumbs grow the header, and
  expanded metadata values wrap within the reading column. Nested indentation and print layout stay separate.
- **`features/progressbar`** — top-of-page loading progress bar (`progressbarStore.ts`, driven
  today by manual `setLoading` calls; a planned TanStack Query migration would drive this from
  `useIsFetching` instead — not yet implemented).
- **`features/router`** — route table and route-level wrappers (`router.tsx`, `lazy-routes.tsx`
  for code-split route bundles, `RouterAuthWrapper.tsx`, `RouterReadOnlyWrapper.tsx` for
  public-access mode).
- **`features/search`** — global search UI (`Search.tsx`, `SearchResultCard.tsx`,
  `HighlightedSearchTitle.tsx`).
- **`features/settings`** — the unified settings page: `SettingsLayout.tsx`, `SettingsNav.tsx`
  (registry-driven section nav), `SettingsSectionGuard.tsx` (route-level role/feature guard),
  `adminSections.ts` (admin-only sections bundled into one lazy chunk),
  `PublicAccessSettings.tsx`, `TocDisplaySettings.tsx`, and `account/` (visible to every user:
  `AccountSettings.tsx`, `PreferencesPanel.tsx` with the language switcher, `AvatarPanel.tsx`,
  `ChangeOwnPasswordPanel.tsx`, `TotpPanel.tsx`).
- **`features/shortcuts`** — keyboard shortcuts help dialog (`ShortcutsDialog.tsx`), consumes
  `lib/shortcuts/shortcutCatalog.ts`.
- **`features/sidebar`** — left navigation sidebar shell and its accordion sections
  (`Sidebar.tsx`, `SidebarAccordionSection.tsx` — the reusable accordion the Pinned/Favorites/Pages
  sections all plug into).
- **`features/snapshot`** — snapshot settings panel (`SnapshotSettings.tsx`).
- **`features/tags`** — tag search result rendering (`TagsResultCard.tsx`).
- **`features/toolbar`** — the top toolbar shell and its per-feature action registration pattern
  (`Toolbar.tsx`, `ToolbarButton.tsx`, `toolbarStore.ts` — most other features register their
  toolbar buttons via their own `useToolbarActions.ts(x)` hook rather than this folder knowing
  about them).
- **`features/tree`** — the page tree view: `TreeView.tsx`, `TreeNode.tsx`, drag-and-drop
  (`TreeDnd.tsx`, `treeDndStore.ts`, `treeDndUtils.ts`), context menu
  (`TreeNodeActionsMenu.tsx`), and the flat pinned-pages sub-list (`PinnedSection.tsx`,
  `PinnedPageItem.tsx` — global/editorial pin, distinct from the per-user favorites star also
  rendered on tree rows).
- **`features/users`** — admin user management (`UserManagement.tsx`, `UserFormDialog.tsx`,
  create/edit/delete buttons and dialogs, `ResendInviteButton.tsx`, admin password change via
  `ChangePasswordDialog.tsx`). A user's own TOTP setup lives in `features/settings/account`.
- **`features/viewer`** — the read-only page view: `PageViewer.tsx`, `Breadcrumbs.tsx`,
  `PageMetadata.tsx`, `EmptySectionChildrenList.tsx` (section-index rendering), scroll-to-heading
  handling.
- **`features/wikilinks`** — `WikiLinkDisambiguationDialog.tsx`, shown when a `[[Title]]` wikilink
  resolves to more than one page — see [ADR-0002: Wikilink Sentinel Encoding](./adr/0002-wikilink-sentinel-encoding.md) for how these
  links are represented internally.

### `stores/*` — top-level (cross-feature) Zustand stores

Per [ADR-0006: Zustand Feature-Local Stores](./adr/0006-zustand-feature-local-stores.md), a store lives here (rather than inside a single
`features/*` folder) when more than one feature needs it directly.

- **`tree.ts`** — the loaded page tree plus `byPath`/`byId` indexes and flattened search items;
  backs both the sidebar tree and the quick switcher.
- **`session.ts`** — current authenticated user (`id`, `username`, `email`, `role`,
  `totpEnabled`), persisted to localStorage.
- **`dialogs.ts`** — which dialog (by string `dialogType` + arbitrary props) is currently open;
  every dialog in `lib/registries/dialogRegistry.ts` is opened through this single store.
- **`config.ts`** — server-reported instance config/feature flags (`publicAccess`,
  `authDisabled`, `enableRevision`, `enableLinkRefactor`, `gitBackupEnabled`, `snapshotEnabled`,
  `maxAssetUploadSizeBytes`, ...), fetched once from `GET /api/config`.
- **`sidebarPanels.ts`** — which sidebar accordion sections (Pinned/Favorites/Pages) are expanded,
  persisted.
- **`sidebar.ts`** — sidebar visibility, active mode (tree vs. search), and width, persisted.
- **`resync.ts`** — resync trigger + poll loop against `GET /api/resync/status`
  (800ms interval, 3-error abort limit) — the frontend half of
  [ADR-0004: Resync Progress via Polling](./adr/0004-resync-progress-via-polling.md).
- **`restore.ts`** — restore trigger (from backup or uploaded ZIP) + status poll loop, same
  poll-interval/error-limit pattern as `resync.ts`.
- **`snapshot.ts`** — snapshot list/trigger/delete + status.
- **`backup.ts`** — git backup status, push/force-push triggers, conflict details for
  `BackupWarningIndicator`.
- **`apikeys.ts`** — API key list/create/delete.
- **`branding.ts`** — logo/favicon/site-name state, including cache-busting version counters
  (`logoVersion`/`faviconVersion`).
- **`editor.ts`** — editor UI state and preferences (preview layout, line wrap, auto-save
  status) — persisted, but distinct from `features/editor/pageEditorStore.ts` which owns the actual
  draft content/dirty state for the page currently open.
- **`userSettings.ts`** — the user's own preferences (auto-save, language, date/time format),
  fetched via `GET /api/user-settings`, not persisted to localStorage (same pattern as
  `favorites.ts`).
- **`avatar.ts`** — avatar state and cache-busting.
- **`lastWikiLocation.ts`** — the last non-settings route, so leaving Settings returns there.
- **`favorites.ts`** — per-user favorited page IDs (`Set<string>`), fetched once via
  `GET /api/favorites`, *not* persisted to localStorage (must be re-fetched per login and cleared
  on logout).
- **`hotkeys.ts`** — global keyboard shortcut registry (`HotKeyDefinition`: key combo, which app
  modes it's active in, action, optional `shouldHandle` guard).
- **`import.ts`** — import plan create/execute/cancel state, wraps `lib/api/import.ts`.
- **`tocPanel.ts`** — collapsed/expanded state of the right-hand "On this page" TOC pane,
  persisted.
- **`users.ts`** — user list/create/update/delete, own-password change.

### `components/ui/*`

shadcn/Radix primitive components (`accordion`, `alert-dialog`, `avatar`, `button`, `checkbox`,
`dialog`, `dropdown-menu`, `input`, `label`, `progress`, `select`, `sonner`, `tabs`, `textarea`,
`tooltip`) — the
mandated building blocks for all form/dialog UI per `AGENTS.md` ("use shadcn primitives instead
of raw HTML form elements; add missing ones here rather than reaching for a plain element"). Composite
components shared across features (`BaseDialog`, `ListView`, `FormInput`, `Pagination`,
`TagInputWithSuggestions`, `RoleGuard`, …) live one level up in `components/`; reuse them before
building new ones.

### `lib/*`

- **`lib/api/*`** — one file per backend domain (`pages.ts`, `auth.ts`, `totp.ts`, `apikeys.ts`,
  `assets.ts`, `avatar.ts`, `backup.ts`, `branding.ts`, `config.ts`, `favorites.ts`, `import.ts`,
  `instanceSettings.ts`, `links.ts`, `properties.ts`, `restore.ts`, `resync.ts`, `revisions.ts`,
  `search.ts`, `snapshot.ts`, `tags.ts`, `userSettings.ts`, `users.ts`), each a thin `fetch`-based client mirroring the matching
  `internal/wiki/<domain>` route surface. `errors.ts` (`mapApiError` — translates backend
  `*LocalizedError` JSON shape into UI-facing errors/field errors).
- **`lib/registries/*`** — `dialogRegistry.ts`/`panelItemRegistry.ts` (typed registries so
  features self-register dialogs/sidebar-panel-items without a central switch statement),
  `index.tsx` (the actual registration calls, plus icon/shortcut wiring), `lazy-dialogs.tsx`
  (code-split dialog imports).
- **`lib/shortcuts/shortcutCatalog.ts`** — canonical list of keyboard shortcuts with metadata,
  consumed by both `features/shortcuts` (help dialog) and `stores/hotkeys.ts`.
- **`lib/i18n.ts`** — `react-i18next` setup. Uses `import.meta.glob('../locales/*/*.json', {
  eager: true })` to build `i18next`'s `resources` from whatever language folders exist under
  `locales/` — a new language needs no code change here. Exports
  `getAvailableLanguages(): { code, name }[]` (feeds the language switcher), where `name` comes
  from each language's own `common.json` → `language.selfName` key.
- Assorted single-purpose helpers at the `lib/` top level: `routePath.ts` (base-path-aware URL
  building), `wikiPath.ts`, `preprocessWikilinks.ts` (`[[Title]]` sentinel handling on the
  frontend side, paired with [ADR-0002: Wikilink Sentinel Encoding](./adr/0002-wikilink-sentinel-encoding.md)), `formatDate.ts`,
  `pageSearch.ts` (flattened tree search index used by the quick switcher), `hotkeys.ts`
  (combo normalization), `useDebounce.ts`, `useIsMobile.ts`, `useAppMode.ts`,
  `useIsReadOnly.ts`, `bootstrapAuth.ts`, `handleFieldErrors.ts`, `deferState.ts`,
  `scrollToHeadline.ts`, `useScrollRestoration.ts`, `useAutoCloseSidebarOnMobile.ts`,
  `redirectToExternal.ts`, `navigationVisit.ts`, `dateLocale.ts`/`useDateTimeFormat.ts` (date
  formatting per user setting), `roles.ts`, `assetSrc.ts`, `dynamicImportError.ts` (recovery
  from stale lazy chunks after a deploy), `config.ts` (frontend-side constant defaults, e.g.
  `DEFAULT_MAX_ASSET_UPLOAD_SIZE_BYTES`). `*.bench.ts` files are Vitest benchmarks.

### `locales/<lang>/*`

One JSON namespace file per feature area (`common.json`, `editor.json`, `settings.json`, …),
duplicated per language folder. `lib/i18n.ts` discovers language folders dynamically, so adding
a language needs no code change. `src/lib/i18n.test.ts` enforces key parity: every namespace
file in every language must have exactly the same key set as English, so a missed translation
fails CI instead of silently falling back. The language switcher (Settings → Account →
Preferences) is generated from the discovered locales. Steps to add a language:
[docs/i18n.md](../i18n.md).

### Other top-level `src/` files

`main.tsx` (React root mount), `App.tsx` (top-level layout/providers, session-driven
favorites load/clear), `App.css`/`index.css` (global styles), `useApplyDesignMode.tsx`
(applies persisted light/dark mode to the document root), `test-setup.ts` (Vitest setup),
`vite-env.d.ts`. `layout/AppLayout.tsx` is the single shared page chrome (toolbar + sidebar +
content area) wrapping all routes.

`main.tsx` bundles self-hosted Inter font faces from `@fontsource/inter`: normal
weights 400–900 for body text, UI labels and prose headings, plus italic
400–700 for emphasis, bold emphasis and blockquotes. `index.css` selects their
`Inter` family through the `--font-sans` theme token. Font files ship with the
frontend; no external font service is needed.

## Not covered here

`e2e/` (Playwright end-to-end tests) is a separate concern from this map.
