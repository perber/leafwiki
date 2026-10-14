# LeafWiki Architecture

See [LeafWiki Source Map](./source-map.md) for the package-by-package directory listing and
[LeafWiki Processes](./processes.md) for end-to-end workflow traces. This page covers layering,
storage, auth and the rendering pipeline.

## Layering

Four layers, strictly one-directional dependency flow:

```
cmd/leafwiki  →  internal/wiki (composition root)  →  internal/wiki/<domain>  →  internal/core/*, internal/<index-package>
```

- **`cmd/leafwiki`** — CLI and flag/env definitions (`cli.go`, `flags_*.go`), log setup, builds a `wiki.WikiOptions`, calls
  `wiki.NewWiki(options)`, wires the HTTP router, handles OS signals
  (`cmd/leafwiki/signals_unix.go` vs `signals_windows.go` — `SIGUSR1`/`SIGHUP` trigger
  `Wiki.TriggerResyncAsync()` on Unix; Windows has no equivalent signal, only the HTTP resync
  endpoint). It owns process lifecycle; it does not know how any domain works internally.
- **`internal/wiki` (composition root)** — `NewWiki()` in `wiki.go` is the single place that
  constructs every store/service and wires them together. This is the file to read to understand
  real init order (see below), not any diagram.
- **Domain slices, `internal/wiki/<name>`** — one package per bounded capability:
  `pages`, `pagesave`, `auth`, `assets`, `avatar`, `revisions`, `search`, `links`, `tags`,
  `properties`, `branding`, `apikeys`, `importer`, `health`, `resync`, `backup`, `snapshot`,
  `restore`, `usersettings`, `instancesettings` (favorites go through `pages`'s use cases, there
  is no `wiki/favorites` package). Each exposes use cases (`New<Verb><Noun>UseCase`) plus a `Routes` HTTP
  registrar built by one `w.build<Name>Routes()` helper in `wiki_routes.go`. Domain slices depend
  downward on `internal/core/*` and the index packages. The only slice other slices import is
  `pagesave` (the shared page-mutation pipeline, used by `pages` and `revisions`); the one other
  exception is `apikeys` reusing `wiki/auth`'s error mapping. Don't add new cross-slice imports:
  cross-domain composition happens in the composition root (e.g. `pagesRoutes` is handed `w.tree`,
  `w.revision`, `w.asset`, `w.favorites`, and a `pagesave.PageSaveOrchestrator` that wires five
  independent side effects together).
- **`internal/core/*`** — the lower-level domain primitives with no knowledge of HTTP or use
  cases: `tree` (page tree + filesystem persistence), `auth` (users/sessions/API
  keys/TOTP/email tokens as SQLite-backed primitives), `markdown` (frontmatter parsing),
  `revision`, `assets`, `email`, `settings` (JSON settings files), `ignore` (`.leafwikiignore`
  resolution), `excerpt`, `shared` (the `sharederrors.LocalizedError` convention lives here),
  `treemigration`, `tools`. Plus sibling top-level packages at the same layer: `internal/search`,
  `internal/links`, `internal/tags`, `internal/properties`, `internal/favorites`,
  `internal/usersettings`, `internal/avatar`, `internal/publicaccess`, `internal/tocdisplay`,
  `internal/branding`, `internal/importer`, `internal/backup`, `internal/snapshot`,
  `internal/restore`.

**Why this shape**: `internal/core/*` packages are storage/domain primitives reusable outside any
one HTTP use case (e.g. `tree.TreeService` is used directly by `importer`, `revision`, and every
`wiki/pages` use case). The `internal/wiki/<domain>` layer exists specifically to hold
*orchestration* — input validation, cross-cutting side effects, logging, metrics — without forcing
that logic into the primitives themselves or into HTTP handlers directly. The composition root (`internal/wiki/wiki*.go`) is deliberately
the *only* place allowed to know about every domain at once; nothing below it may reach sideways
into a sibling domain package.


### Real init order (`NewWiki()` in `wiki.go`, the `init*` steps in `wiki_boot.go`)

```
initAuth               → users.db, sessions.db (if auth enabled), TOTP service (if key set),
                          api_keys.db (if EnableAPIKeyManagement)
initEmail              → email_tokens.db + SMTP service (only if SMTP is configured)
initCoreServices       → ignore.Cache, tree.TreeService.LoadTree() (sync — routing needs the tree),
                          SlugService, AssetService
initLinkService        → links.db + synchronous IndexAllPages() (blocks NewWiki())
initTagsService        → tags.db (store only)
initPropertiesService  → properties.db (store only)
initFavoritesService   → favorites.db
initUserSettingsService → usersettings.db
bootstrapTagsAndProperties() → clears + rebuilds tag/property indexes in one parallel GetPages pass
initSearch             → search.db + an ASYNC goroutine that reindexes everything in the background
                          (IndexingStatus tracks progress; NewWiki() does NOT wait for it)
initBranding           → branding config
initAvatarService      → avatar storage
EnsureWelcomePage()    → creates the seed page through the normal orchestrator if the tree is empty
[if EnableRevision]    → revision.Service + ensureBaselineRevisions() (one baseline revision per
                          existing page, so revision history has a starting point)
buildRoutes()          → wires every domain's use cases + Routes registrar
```

This is currently a mix of synchronous work (tree, links, tags/properties all block `NewWiki()`
returning) and one asynchronous exception (search runs its full reindex in a background goroutine).
That asymmetry, and the fact that *all* of these rebuild unconditionally on every boot
regardless of whether the on-disk indexes already match, is a known optimization target: boot
could skip the rebuild when the persisted indexes are trustworthy.

`Wiki.Close()` (`wiki.go`) cancels the shutdown context and drains in-flight reload goroutines
(`reloadWG`) first, then closes the stores (auth, users, API keys, email tokens, links, user
settings, favorites, …) and finally the search index, so no reload goroutine still touches a
closed store.

## Storage model

Markdown files under `<data-dir>/root/` are the single source of truth, owned exclusively by
`internal/core/tree` (page content, frontmatter, hierarchy via directory structure) and
`internal/core/revision` (content-addressed blobs + JSON metadata for revision history) — see
[ADR-0001: Filesystem as Source of Truth](./adr/0001-filesystem-source-of-truth.md). Every SQLite database beside them falls into one of two
categories:

**Derived indexes — rebuildable from the filesystem, safe to lose:**

| Store | Package | File |
|---|---|---|
| Full-text search | `internal/search` | `search.db` |
| Wikilink graph | `internal/links` | `links.db` |
| Tag index | `internal/tags` | `tags.db` |
| Frontmatter property index | `internal/properties` | `properties.db` |

These four are exactly what `internal/wiki/resync` (triggered via `POST /api/admin/resync`,
`SIGUSR1`/`SIGHUP`, or automatically at every `NewWiki()` boot today) rebuilds. The core sequence —
`tree.ReconstructTreeFromFSContext` → `links.IndexAllPagesContext` →
`bootstrapTagsAndProperties()` → search reindex — appears twice in the current codebase:
the synchronous boot path inside `NewWiki()` and `reloadWithProgress`
(`wiki_resync.go`, the one used by both the admin resync endpoint and the OS-signal handler, phase-
tracked via `wikiresync.ResyncJob` per [ADR-0004: Resync Progress via Polling](./adr/0004-resync-progress-via-polling.md)).

**Real state — NOT derived from disk, losing them is real data loss:**

| Store | Package | File |
|---|---|---|
| Users (accounts, roles, password hashes, TOTP secrets) | `internal/core/auth` | `users.db` |
| Active JWT refresh sessions | `internal/core/auth` | `sessions.db` |
| API keys (hashed) | `internal/core/auth` | `api_keys.db` |
| Favorited pages per user | `internal/favorites` | `favorites.db` |
| Per-user preferences | `internal/usersettings` | `usersettings.db` |
| Password-reset / invite tokens | `internal/core/auth` | `email_tokens.db` |

Plus a few small JSON settings files in the data directory (`branding.json`,
`public-access.json`, …, via `internal/core/settings`) and the asset/avatar/branding files on
disk. Global page pins are not in a database: they live in the page's frontmatter.

`reloadWithProgress`'s doc comment states this explicitly: "What is NOT reloaded:
assets (served directly from disk), auth/users (SQLite-based), revisions (stored per-page, no
re-baseline)." Resync never touches any of the real-state stores — there is no mechanism that rebuilds them
from anything else, because nothing
else contains the information needed to reconstruct them. This is precisely why full backup/restore
(`internal/snapshot`, `internal/restore`) has to explicitly
enumerate and copy these files rather than relying on resync. `sessions.db` is the deliberate
exception: session state is ephemeral, so restore invalidates all sessions instead of restoring
them (`swapNames` in `internal/restore/swap.go`).

Why the distinction matters operationally: a corrupted or deleted derived index is a `resync` away
from full recovery with zero data loss. A corrupted or deleted real-state store is unrecoverable
except from a backup — there's no filesystem walk that can regenerate a lost password hash or a
revoked-but-still-listed API key.

## Auth model

Three independent front doors, all converging on the same underlying `*auth.User` /
`internal/core/auth` role model (`admin` / `editor` / `viewer`, plus public read-only access when
`--public-access` is set):

1. **Native username/password**, optionally hardened with per-user TOTP two-factor. Handled by
   `auth.AuthService` (`internal/core/auth/auth_service.go`), which after
   [ADR-0009: Session Manager Extraction](./adr/0009-session-manager-extraction.md) holds password/TOTP verification only —
   `Login`, `CompleteTOTPLogin`, TOTP setup/disable — and delegates all session mechanics to a
   separate `SessionManager`.
2. **JWT sessions**, via `SessionManager` (`internal/core/auth/session_manager.go`):
   `IssueSession`, `RefreshToken`, `RevokeRefreshToken`, `RevokeAllUserSessions`,
   `RevokeAllUserSessionsExceptCurrent`, `ValidateToken`, backed by `sessions.db`. This extraction
   exists specifically as the seam for a planned-but-not-yet-built native OIDC provider: a future
   `OIDCService` would resolve/provision a `*User` and then call the same
   `sessionManager.IssueSession(user)` that password login calls today, reusing token
   issuance/refresh/revocation as-is rather than each identity provider inventing its own session
   handling.
3. **Reverse-proxy header trust**, via `InjectRemoteUser` middleware
   (`internal/http/middleware/auth/reverse_proxy.go`, gated by `--enable-http-remote-user` + a
   trusted-header name + a trusted-proxy IP allowlist). When active, it short-circuits normal JWT
   validation for the request by trusting an already-authenticated header from an SSO reverse proxy
   (Authelia, Authentik, oauth2-proxy, Tailscale) — see
   [ADR-0005: Reverse-Proxy Header Auth](./adr/0005-reverse-proxy-header-auth.md). Originally required the asserted user to already exist;
   [ADR-0010: Remote User Auto-Provisioning](./adr/0010-remote-user-auto-provisioning.md) added an opt-in extension
   (`--enable-http-remote-user-auto-create`, separate flag, off by default) that calls
   `UserService.GetOrCreateRemoteUser` to mint a new account on first sight — with a
   cryptographically random password the account can never log in with directly, and a default role
   that can never be `admin`, keeping a forged-header blast radius bounded.

There is also a fourth, narrower path: **Bearer API keys**
(`internal/http/middleware/auth/apikey.go`, `InjectAPIKeyUser`, feature-flagged via
`EnableAPIKeyManagement`), which authenticate as the key's owning user with a role that can only
narrow (never widen) that user's role — currently read-only in practice, since API-key requests
carry no CSRF cookie and every mutating endpoint requires one.

**How they coexist**: all four paths resolve to the same `*auth.User`/role model in
`internal/core/auth`, so a proxy-trusted user and a password-login user are not different kinds of
account — a user record created via `GetOrCreateRemoteUser` is a normal row in `users.db`, just one
whose password is unusable. A proxy-trusted user and a locally-password-authenticated user can
coexist in the same instance; nothing prevents an admin from also having a real password set
independent of how a given request happened to authenticate. Role/permission checking
(`RequireAdmin`, editor/admin gating in `internal/http/middleware/auth/auth.go`) is uniform
downstream of authentication — it reads the resolved user's role regardless of which of the three
front doors established `CurrentUser` for that request.

## Markdown-to-HTML rendering pipeline (raw HTML safety)

Can an unclosed `<div>` or other malformed HTML in a page's markdown break the surrounding app
layout (sidebar, header, other pages)? No.

Both render targets — the read-only viewer (`PageViewer.tsx`) and the live editor preview
(`MarkdownEditor.tsx`) — share one component, `ui/leafwiki-ui/src/features/preview/MarkdownPreview.tsx`,
with this plugin pipeline:

```
remarkMath, remarkGfm, remarkFlexibleMarkers, remarkImageSize
→ rehypeRaw               (parses embedded HTML strings into hast nodes)
→ rehypeLineNumber, rehypeCodeFenceLineNumbers
→ rehypeWhitelistStyles   (filters inline `style` to a property whitelist)
→ rehypeKatex
→ rehypeSanitize          (custom schema, sanitizes the WHOLE tree incl. rehypeRaw's output)
→ rehypeHighlight
```

Key facts:

- Between `rehypeRaw` and `rehypeWhitelistStyles`, `rehypeLineNumber` and
  `rehypeCodeFenceLineNumbers` add source-line data attributes (used for editor ↔ preview scroll
  sync); `remarkImageSize` handles image size syntax.
- `rehypeRaw` doesn't inject a raw string into the DOM — it parses via `hast-util-raw`/`parse5`,
  the same spec-compliant HTML5 tree-construction algorithm browsers use. Unclosed tags get
  auto-closed exactly like a browser would; the result is always a well-formed tree. No
  `dangerouslySetInnerHTML` anywhere in the frontend.
- Sanitize order is correct: `rehypeSanitize` runs *after* `rehypeRaw`, so raw HTML is never left
  unsanitized (this is the safe order — reversed, it would be an XSS hole).
- `rehypeWhitelistStyles.ts` allows layout/typography properties (margin, padding, width/height,
  color, border, display, …) but **not** `position`, `z-index`, or `top/left/right/bottom` — so
  inline styles can't be used to escape the content container via `position: fixed` etc.
- Both render targets wrap content in scoped containers (`.page-viewer__content`,
  `.markdown-editor__preview`, both `prose`-classed). Since parse5 guarantees proper nesting, a
  malformed tag can mis-render *within* that container's own subtree (wrong nesting, a box
  swallowing trailing content, a broken table) but has no structural path to bleed into sidebar,
  header, or other pages.

**Worth a periodic audit:** the custom `rehypeSanitize` schema extends `defaultSchema` to allow
`style`, `class`, `audio`/`video` `src`, and custom `wikilink-*` protocols. Check that these additions don't reopen a CSS-injection or protocol-based vector when the
schema changes.
