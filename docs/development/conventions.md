# Conventions

How code in this repository is shaped, so new code looks like the code around it. Most rules
describe what the code already does; where the code doesn't follow a rule yet, the exception is
listed and has a cleanup plan.

The short list of rules that matter most is in [rules.md](./rules.md); this page is the
reference behind it. Look up the section for the area you are working in.

## Goal

A codebase that stays maintainable while it gains features. When rules pull in different
directions, the order is: **correctness and security → extensibility → readability and
maintainability → speed of delivery.** A feature that works but closes the architecture or makes
the code harder to change is not done.

## Layers and where code goes

```
cmd/leafwiki → internal/wiki (composition root) → internal/wiki/<slice> → internal/<package>, internal/core/*
```

| Layer | Holds | May import |
|---|---|---|
| `internal/core/*` | Storage and domain primitives: tree, auth, revision, markdown, assets, settings, … | other `internal/core/*`, stdlib, third-party. **No** gin, no `internal/wiki`, no `internal/http`, no top-level `internal/*` packages |
| `internal/<package>` (links, search, tags, properties, favorites, usersettings, backup, snapshot, restore, …) | Derived indexes and optional feature services built on core | `internal/core/*`, other top-level packages. **No** `internal/wiki`, no `internal/http` |
| `internal/http/**` | Router infrastructure, middleware, DTOs | everything below. **No** `internal/wiki` |
| `internal/wiki/<slice>` | One capability's use cases + HTTP routes | layers below, plus `internal/wiki/pagesave`. **No** other slice |
| `internal/wiki` (`wiki.go`, `wiki_boot.go`, `wiki_routes.go`, `wiki_resync.go`) | Composition root: builds and wires everything | everything |

`internal/architecture_test.go` enforces these import rules.

### What belongs in `internal/core`

A package goes into `internal/core` only if **all** of these hold:

1. It owns persistent state or a format (files on disk, a SQLite store, the Markdown/frontmatter
   format) or is a pure primitive other core packages need.
2. It knows nothing about HTTP, requests, roles or the current user's permissions. User IDs as
   data (authorship) are fine; deciding what a user may see or do is not.
3. It is needed by more than one slice or package, or by another core package.
4. It has no optional-feature switches. A feature that can be turned off belongs in a top-level
   `internal/<package>`.

Otherwise: a derived index or optional feature service → top-level `internal/<package>`;
orchestration of several services or anything HTTP-shaped → `internal/wiki/<slice>`.

**Cross-cutting rules (visibility, drafts, access control, …) do not go into `core/tree`.** The
tree stays a plain structure store; such rules sit in a layer above it that all readers go
through. Until that layer exists, don't add `if`s for them to `TreeService`; open an issue
instead.

## Extensibility (strict)

LeafWiki follows the SOLID principles, with the **Open/Closed Principle** as the leading rule:
code is open for extension, closed for modification. A new feature is added by **plugging into
a seam**, not by editing shared code to know about it. Reviews reject changes that close the
architecture, even when they work.

| Principle | What it means here |
|---|---|
| **Open/Closed** | New features register at an extension point; shared code doesn't change to learn about them (rules 1–3 below). |
| **Single Responsibility** | One use case per operation, one slice per capability; handlers only translate HTTP, use cases orchestrate, core stores. |
| **Liskov Substitution** | Every implementation of a seam (`PageSideEffect`, `Provider`, a registry entry) must work wherever the seam is used: same error contract, no hidden preconditions, no panics the caller doesn't expect. `Managed` and `Fixed` settings are interchangeable. |
| **Interface Segregation** | Consumers define small interfaces with only the methods they call (`importer.ImporterWiki`, `tocdisplay.Provider`), not a dependency on `*wiki.Wiki` or a whole service. |
| **Dependency Inversion** | Slices and features depend on interfaces; the composition root (`wiki.go`, `cmd/leafwiki`) picks and wires the implementations. |

**The test for every change:** could a second feature of the same kind be added later without
touching this code again? If not, the change needs an extension point first.

Rules:

1. **Use an existing extension point.** Today:

   | Seam | Where | Used for |
   |---|---|---|
   | `RouteRegistrar` | `internal/http/registrar.go` | each slice registers its own routes |
   | `PageSideEffect` | `internal/wiki/pagesave` | reactions to page create/update/move/delete/restore |
   | `Provider` interfaces + `settings.Value` (`Managed`/`Fixed`) | `internal/publicaccess`, `internal/tocdisplay`, `internal/core/settings` | instance settings changeable at runtime or pinned by a flag |
   | Narrow consumer interfaces | e.g. `importer.ImporterWiki` | a package depends on what it needs, not on `*wiki.Wiki` |
   | Dialog, panel-item and settings-section registries | `ui/leafwiki-ui/src/lib/registries/` | dialogs, sidebar panels, settings pages |
   | Toolbar actions | `useToolbarActions` per feature | toolbar buttons |

2. **No fitting seam → add the extension point first**, in its own PR, with the existing
   behaviour as its first user and no behaviour change. Then build the feature on it. A new
   extension point gets an ADR (direction: [ADR-0007](./adr/0007-extension-points-long-term-goal.md)).
3. **Central code registers, it doesn't branch.** The composition root (`internal/wiki/wiki*.go`), `internal/http/router.go`,
   `TreeService`, `MarkdownPreview`, `SettingsNav`, `DialogManager` and similar shared places
   must not grow `if feature X` / `switch kind` chains. Adding a feature means adding a
   registration, not editing logic. Conditional registration in the composition root (`if
   enabled { register(...) }`) is fine.
4. **Feature state stays in the feature.** Don't add feature-specific fields to shared
   structures (`PageNode`, the tree DTO, `User`); keep the data in the feature's own package or
   store, or as frontmatter the tree passes through without interpreting it. (`PageNode.Pinned`
   is an existing exception.)
5. **Cross-cutting rules (visibility, drafts, access control) never go into `core/tree` or into
   individual handlers.** They belong in one layer above the tree that every reader goes
   through. Until that layer exists, such a feature starts by building it.
6. **Depend on narrow interfaces at seams**, defined by the consumer ("accept interfaces"), so
   implementations can be swapped or added without touching the consumer.
7. **Optional features can be switched off**: a flag or setting, read in one place (the
   composition root or a `Provider`), never checked all over the code.

`internal/architecture_test.go` enforces the import side of this; the rest is checked in review.

## Reuse: extract what is used more than once

When the **same logic** shows up a second time — code that would have to change together, not
code that only looks similar — extract it instead of copying it. Two copies are a smell; the
third copy is not allowed. Coincidental similarity (two validators that happen to check the same
length today) stays separate.

**Where the extracted code goes depends on what it needs**, and the layer rules decide:
a shared helper must not import the packages that use it.

| Used by | Depends on | Goes to |
|---|---|---|
| one file / one package | anything the package has | unexported helper in that package |
| several slices or packages of one domain | that domain | exported from the domain package (e.g. `internal/links`) or a sub-package of it |
| any package, no domain knowledge | stdlib or a small library only | `internal/core/shared/<topic>` (one topic per package, like `htmlutil`, `sqliteutil`, `crypto`, `errors`); don't grow `shared/utils.go` into a grab bag |
| HTTP-shaped code (responses, request parsing) | gin, `net/http` | `internal/http/...` |
| tests of one package | the package | its fixture file (`fixture_test.go`) |
| tests of several packages | test infrastructure | `internal/test_utils` |

If the extracted code would need a dependency on a higher layer, it is in the wrong place:
define a small interface where it is used and let the composition root pass the
implementation.

Frontend, same idea:

| Used by | Goes to |
|---|---|
| one component | the component file |
| one feature | the feature folder |
| several features, pure TypeScript (no React) | `lib/<topic>.ts` |
| several features, a hook | `lib/useX.ts` |
| several features, a component | `components/` |
| generic UI primitive (shadcn) | `components/ui/` |

Features don't import another feature's components or helpers sideways; what two features need
moves up a level. (Calling another feature's store for cross-feature effects is fine,
[ADR-0006](./adr/0006-zustand-feature-local-stores.md).)

Extracting is a refactoring: its own commit, behaviour-preserving, with a test for the extracted
unit. Larger extractions (several packages) are their own PR.

## Backend (Go)

### Slice layout

- `routes.go` — the `Routes` struct (`RouteRegistrar`), route registration and handlers.
- `use_cases.go` — the use cases, one block per use case, separated by
  `// ─── XxxUseCase ──────` lines, in the same order as the routes in `routes.go`.
- `errors.go` — the slice's error codes and their HTTP status mapping.
- **From 6 use cases on**, split `use_cases.go` into one file per use case named
  `<verb>_<noun>.go` (see `internal/wiki/pages`).

### Use case shape

```go
type MovePageInput struct { … }
type MovePageOutput struct { … } // omit when there is nothing to return

type MovePageUseCase struct { tree *tree.TreeService; … }

func NewMovePageUseCase(t *tree.TreeService, …) *MovePageUseCase { … }

func (uc *MovePageUseCase) Execute(ctx context.Context, in MovePageInput) (*MovePageOutput, error)
```

`ctx` is always the first parameter, also when unused. Input is always a struct (not
positional arguments), so adding a field doesn't change every call site.

Handlers only bind and validate the request, resolve the user, call `Execute` and write the
response. Business rules live in the use case or below.

### Order inside a file

Package doc → imports → constants and error codes → exported types → constructor → exported
methods → unexported helpers. Exported identifiers have a doc comment.

### Small idioms

- Ignored close/cleanup errors are explicit: `defer func() { _ = f.Close() }()`.
- No testify; stdlib `testing` only (see [Tests](#tests)).

### Long-running work

Accept a `ctx`, check `ctx.Err()` between phases and return it (see `runReloadPhases` in
`internal/wiki/wiki_resync.go`).

## Errors

### Error codes

- Format: `<domain>_<reason>`, snake_case, e.g. `page_not_found`, `api_key_invalid_role`.
- `<domain>` is the slice name in singular snake_case: `page`, `asset`, `auth`, `api_key`,
  `backup`, … One domain prefix per slice.
- `<reason>` uses these suffixes where they fit; the shared error helper derives the HTTP status
  from them (until it exists, map them the same way in the slice's `errors.go`):

  | Suffix | Status | Example |
  |---|---|---|
  | `_invalid_<what>`, `_missing_<what>` | 400 | `asset_invalid_name` |
  | `_unauthorized`, `_invalid_credentials`, `_token_expired` | 401 | `auth_token_expired` |
  | `_forbidden`, `_disabled` (feature switched off by config) | 403 | `auth_email_disabled` |
  | `_not_found` | 404 | `page_not_found` |
  | `_already_exists`, `_conflict`, `_already_running`, `_version_conflict` | 409 | `page_version_conflict` |
  | `_too_large` | 413 | `asset_file_too_large` |
  | `_unavailable` (temporary, e.g. a store suspended during restore) | 503 | `auth_user_store_unavailable` |
  | `_internal_error`, `_failed` | 500 | `asset_upload_failed` |

- Codes are part of the API contract: never rename or reuse one. Clients (the frontend, API-key
  users) branch on `code`, never on `message`.

### Error response body

Every error response uses one envelope:

```json
{
  "error": {
    "code": "page_not_found",
    "message": "Page not found",
    "template": "page %s not found",
    "args": ["abc123"],
    "fields": [
      { "field": "slug", "code": "page_invalid_slug", "message": "…", "template": "…", "args": [] }
    ]
  }
}
```

- `message`: English, for logs and API clients.
- `template`: the i18n key in the frontend's `errors` namespace (`locales/<lang>/errors.json`),
  `%s` placeholders filled from `args` in order. Every template used by the backend has a key
  in every locale.
- `fields`: only for validation errors (`code: "validation_error"`, status 400), one entry per
  invalid field, each with its own code and template.

Backend: domain errors are `*sharederrors.LocalizedError`; field errors are
`*sharederrors.ValidationErrors`.

**Current state:** each slice still has its own copy of the response structs and status
switch; validation errors are sent as `{"error": "validation_error", "fields": [...]}` with
untranslated messages; some codes don't follow the naming yet (`api_keys_disabled` and
`apikey_store_unavailable` next to `api_key_*`, `usersettings_store_unavailable`,
`restore_writes_disabled` for a temporary 503). Both are being moved onto one shared helper; until then, copy the shape of
the slice you are in, don't invent a new one.

## HTTP API

- Base path `/api`. Resources are plural nouns in kebab-case (`/api/api-keys`,
  `/api/user-settings`); IDs are path parameters (`:id`). JSON fields are camelCase.
- Actions on a resource go after the ID: `POST /api/pages/:id/move`, `/pin`, `/refactor/apply`.
- Methods: `GET` reads, `POST` creates or triggers an action, `PUT` updates, `DELETE` deletes.
  No `PATCH` without an ADR.
- Status: 200 with body, 201 created, 202 async job started, 204 no body, plus the error statuses
  above.
- Async jobs: `POST …` returns 202, then the client polls `GET …/status`
  ([ADR-0004](./adr/0004-resync-progress-via-polling.md)). Used by resync, restore, search
  indexing, snapshots.
- Every route states its role middleware explicitly; mutating routes go through CSRF.
- The API is reachable from outside via API keys. Renaming a route or changing a response
  shape is a breaking change: keep the old route as an alias for at least one minor release and
  note it in the release notes.

**Known outliers** (to be migrated with an alias period, don't copy them):

| Today | Target |
|---|---|
| `POST /api/pages/copy/:id` | `POST /api/pages/:id/copy` |
| `POST /api/pages/convert/:id` | `POST /api/pages/:id/convert` |
| `/api/snapshot…` | `/api/snapshots…` |
| `/api/user/avatar` | `/api/users/me/avatar` |
| `PUT /api/pages/:id/assets/rename` body `old_filename`/`new_filename` | `oldFilename`/`newFilename` |

## Frontend

- `lib/api/<domain>.ts` mirrors one backend slice: `fetchX` / `createX` / `updateX` /
  `deleteX`, optional `signal?: AbortSignal` as the last parameter, all through
  `fetchWithAuth`. Pass the response type as type parameter (`fetchWithAuth<TagCount[]>(…)`)
  instead of casting the result; it is a compile-time type, not a runtime check, so validate
  where a malformed server response would be dangerous. Error bodies are decoded centrally
  (`lib/api/errors.ts`); wrappers don't parse errors themselves.
- State per [ADR-0006](./adr/0006-zustand-feature-local-stores.md): hooks are named
  `useXxxStore`; files are `xxxStore.ts` inside a feature folder, `<domain>.ts` in `stores/`
  when several features need it.
- One exported component per file, PascalCase (`PageViewer.tsx`); hooks `useX.ts`; tests next
  to the file (`X.test.tsx`).
- Order inside a component file: imports → types → constants → component → local helpers.
- No raw HTML sinks (`dangerouslySetInnerHTML`, `innerHTML`/`outerHTML`, `insertAdjacentHTML`,
  `document.write`, `srcDoc`, `eval`, `new Function`); the ESLint XSS guard enforces it. HTML
  reaches the DOM only through the sanitized Markdown pipeline in `features/preview`. A justified
  exception gets `eslint-disable-next-line` plus a comment explaining why the input is safe.
- Reuse `components/` (`BaseDialog`, `ListView`, `FormInput`, …) and `components/ui/` before
  building something new. Strings via i18n, form controls via shadcn (see `AGENTS.md`).

## Cross-platform

LeafWiki ships for Linux (amd64, 386, arm64), Windows, macOS and Raspberry Pi as one binary.

- `filepath` for OS paths, `path` for URL and slug paths; never hardcode `/` or `\`.
- Don't assume a case-sensitive filesystem (Windows and macOS aren't): two names that differ
  only in case are the same file.
- OS-specific code goes into build-tagged files (`cmd/leafwiki/signals_unix.go` vs
  `signals_windows.go`, `internal/restore/selfrestart_*.go`).
- No cgo. Keep memory and CPU use reasonable for a Raspberry Pi.
- Check that it builds for Windows and macOS: `make cross` (build and vet for
  windows/amd64 and darwin/arm64). *Enforced by the `build-go-cross` job in
  `.github/workflows/backend.yml`, which runs the same target.*

## Naming

| What | Scheme | Examples |
|---|---|---|
| CLI flag | kebab-case, `<area>-<setting>`; every flag of one feature shares the area prefix | `smtp-host`, `git-backup-branch`, `snapshot-retention` |
| Env var | exactly `LEAFWIKI_` + the flag in upper snake case, nothing else | `--smtp-host` → `LEAFWIKI_SMTP_HOST` |
| Boolean flag, opt-in feature (default off) | `enable-<feature>` | `enable-metrics`, `enable-link-refactor` |
| Boolean flag, opt-out (default on) | `disable-<thing>` | `disable-request-log` |
| Boolean flag, loosens security (default off) | `allow-<thing>` | `allow-insecure` |
| Duration | Go duration string; suffix `-timeout` (max wait), `-interval` (period), `-window` (span); `0` = off, documented | `smtp-timeout`, `snapshot-interval`, `revision-coalesce-window` |
| Size / count limit | `max-<thing>` prefix, unit named in the help text | `max-asset-upload-size`, `max-revision-history` |
| Path | suffix `-dir` (directory) or `-path` (file) | `data-dir`, `git-backup-ssh-key-path` |
| URL | suffix `-url` | `public-url`, `logout-url` |
| Frontmatter key (reserved) | `leafwiki_` + snake_case; never reused for something else | `leafwiki_id`, `leafwiki_pinned` |
| Data files | kebab-case `.json` for settings, `<name>.db` for SQLite, flat in the data dir | `public-access.json`, `users.db` |
| Prometheus metric | `leafwiki_` + snake_case, unit suffix (`_seconds`, `_total`, `_bytes`) | `leafwiki_http_requests_total` |
| Log field | camelCase ([ADR-0008](./adr/0008-structured-logging-schema.md)) | `pageID`, `oldSlug` |
| Error code | `<domain>_<reason>` (see [Errors](#errors)) | `page_not_found` |
| i18n key | `<area>.<element>` camelCase (see [i18n](#internationalization-i18n)) | `page404.createButton` |
| Branch | `<type>/<short-topic>`, type as in Conventional Commits; issue number if there is one | `fix/1656-attachments-visibility`, `refactor/avatar-use-case-input` |

Existing outliers, kept for compatibility: boolean flags without `enable-`/`disable-`/`allow-`
(`git-backup`, `snapshot`, `public-access`, `hide-link-metadata-section`,
`smtp-insecure-skip-verify`), `editor-limit`, `restore-upload-max-size`. New flags follow the
table. The env var scheme and the boolean prefixes are enforced by
`cmd/leafwiki/flags_naming_test.go`; its `legacyBoolFlagNames` allowlist may only shrink.
Every flag is documented in the configuration reference under `docs/` and listed
(commented out) in `.env.example` in the same PR.

## Dependencies

A new dependency (Go module or npm package) needs a reason in the PR description:

- **What it replaces** and why the stdlib / an existing dependency / ~50 lines of own code
  don't do.
- **Weight:** binary size (Go) or bundle size and whether it can be lazy-loaded (frontend).
- **Health:** maintained, licence compatible (MIT/BSD/Apache-style), no large transitive tree.
- **Platforms:** works on linux/amd64, 386, arm64, Windows and macOS without cgo.

A dependency with lasting weight (a runtime service, a database, a framework, a protocol) needs
an ADR. Dependabot bumps are not new dependencies.

Toolchain versions have one source each: Go in the `toolchain` line of `go.mod`, Node in
`.nvmrc`; CI reads both. Dependabot bumps only the `golang`/`node` base images in the
Dockerfiles, so such a PR updates the matching file too. *Enforced by `make verify`
(`scripts/check-toolchain-versions.sh`): the full Go version must match, so the Dockerfiles
pin a patch tag such as `golang:1.27.3-alpine`; Node must match in the major version.*
CI jobs call Makefile targets; exceptions are listed at the `ci` target.
Trivy scans the dependencies (`go.sum`, `package-lock.json`) on every PR, and the final Docker
image on PRs that change an image input (`make scan-image`) and weekly against the published
image (`make scan-published-image`), both in `.github/workflows/image-scan.yml`. Fixable
HIGH/CRITICAL findings fail the scan; a false positive goes into `.trivyignore` with a comment
saying why.

## On-disk format and migrations

The Markdown tree and its frontmatter are the user's data ([ADR-0001](./adr/0001-filesystem-source-of-truth.md)).
A change to how pages, frontmatter, folders or links are stored on disk:

1. bumps `CurrentSchemaVersion` (`internal/core/tree/schema.go`) and adds the step to
   `internal/core/treemigration` (`migrationForVersion`); each step migrates exactly one version,
2. is **idempotent** and tested from the previous version's real layout (fixture tree in
   `testdata/`) to the new one, including a run on an already migrated tree,
3. never silently drops or rewrites user content it doesn't understand,
4. has an ADR and a release note that tells admins to take a backup before upgrading.

Starting an older binary on a newer schema fails with a clear error; keep that behaviour.
Derived indexes (SQLite) don't need a migration: bump their schema and rebuild via resync.

A new SQLite store chooses a `CorruptionPolicy` at its constructor
([ADR-0013](./adr/0013-per-store-sqlite-corruption-policy.md)): derived index →
`RebuildOnCorruption`; real state → `FailOnCorruption` or `QuarantineOnCorruption`, never Rebuild.

## Deprecation

Anything users or API clients depend on — routes, request/response fields, error codes, CLI flags,
env vars, frontmatter keys — is not removed or renamed in one step:

1. Add the new form; keep the old one working for **at least one minor release**.
2. The old form logs a warning once per process (`… is deprecated, use … instead`; see
   `resolveLogoutURL` in `cmd/leafwiki/main.go`) and is marked deprecated in `--help` / the
   docs.
3. Release notes of both releases name the change. Then remove the old form.

## Feature flags

- Unfinished work behind a flag uses the naming above (`enable-<feature>`, default off,
  "(experimental)" in the help text).
- Every experimental flag has a tracking issue. When the feature is done, the flag either
  becomes a documented, permanent option or is removed — **at the latest two minor releases
  after the feature is complete**.
- The flag is read in one place (the composition root or a `Provider`) and passed down; code
  doesn't check it in several places.

## Concurrency and background work

- Every request and long operation takes a `ctx` and passes it down; loops over pages check
  `ctx.Err()`.
- Goroutines are owned: started through a function that registers them (a `WaitGroup` like
  `reloadWG` in `internal/wiki`), stopped by the shutdown context, and waited for in `Close()`.
  No fire-and-forget goroutine that can outlive a test or the process.
- No file or network I/O while holding the tree lock unless the operation requires it (writing
  the page itself); prepare outside, apply inside.
- New code with shared state gets a test that runs with `go test -race`.

## Internationalization (i18n)

Every user-facing string in the UI is translatable; wiki page content is not. How to add a
language: [docs/i18n.md](../i18n.md). How to use i18n in code:

### Where strings live

- One **namespace per feature**: `ui/leafwiki-ui/src/locales/<lang>/<namespace>.json`, named
  like the feature in camelCase (`editor`, `publicAccess`, `brokenLinks`). Shared strings
  (buttons like Cancel/Save, generic dialogs) go into `common`. A new feature gets its own
  namespace; don't append unrelated keys to someone else's.
- **Every key exists in every locale** (`src/lib/i18n.test.ts` enforces it). Add the English text
  in `en/` and the same key in all other locales: a real translation if you can, otherwise the
  English text so the parity test passes and translators can find it.

### Keys

- camelCase, grouped by where the string appears: `<area>.<element>`, at most three levels
  (`unsavedChangesDialog.leaveAnywayButton`, `page404.createButton`).
- Name the role, not the wording: `…Button`, `…Title`, `…Description`, `…Placeholder`,
  `…Tooltip`, `…SrOnly` (screen-reader-only), `…Toast`, `…Error`.
- **Exception — `errors`:** keys are the backend's English `template` strings (`"asset %s not
  found"`), because the backend sends them in the error envelope (see [Errors](#errors)). Every
  template the backend can send has a key in `errors.json` in every locale.

### In code

- React: `const { t } = useTranslation('<namespace>')`, always with the namespace. Outside
  React: `i18next.t(key, { ns: '<namespace>' })`.
- **Never build sentences from pieces** (`t('deleted') + ' ' + name`). Use interpolation:
  `"deletedPage": "Deleted {{title}}"` → `t('deletedPage', { title })`. Word order differs
  between languages.
- **Plurals** via `count` and i18next suffixes: `"pages_one": "{{count}} page"`,
  `"pages_other": "{{count}} pages"`, `t('pages', { count })`. Never `count === 1 ? … : …`.
- **Markup inside a sentence** (a link, bold text) via `<Trans>` with named components, never
  HTML in the translation string passed to `innerHTML`.
- Interpolation is not HTML-escaped by i18next (`escapeValue: false`); React escapes on render.
  So translated strings are rendered as React text only, never as raw HTML.
- **Dates and times** via `useDateTimeFormat()` / `lib/dateLocale.ts` (respects the user's date
  format setting), numbers via `Intl.NumberFormat(bcp47(lng))`. No hand-formatted dates.
- `aria-label`, `title`, `placeholder` and toast texts are user-facing too and go through `t`.

### Backend

- The backend does not translate. User-facing errors carry a stable `code`, an English
  `message` and a `template` that the frontend translates via `errors.json`.
- Supported languages: `internal/usersettings/language.go` keeps an allow-list that must match
  the locale folders; adding a language means adding it there too (see docs/i18n.md).
  *Enforced by `TestAllowedLanguages_MatchShippedUILocales`.*
- Emails (password reset, invite) are English only today.

## Tests

Tests are code that has to be read and maintained. Organize them so a reader finds the case
they are looking for and sees at a glance what is covered.

### Organization

- **One test file per unit of behaviour**, next to the code (`move_page.go` →
  `move_page_test.go`, `PageViewer.tsx` → `PageViewer.test.tsx`). A test file past ~800 lines
  is split by capability. Known oversized files to split (each its own PR):
  `internal/http/router_test.go`, `internal/wiki/pages/pages_test.go`,
  `internal/core/tree/tree_service_test.go`.
- **Test at the right layer.** HTTP tests check routing, auth, CSRF, status codes and the JSON
  shape; use-case/domain tests check state changes, versions and side effects; a few end-to-end
  tests check that they fit together. Don't re-test domain rules through HTTP.
- **Fixtures:** one default fixture per package with explicit overrides
  (`newTestDeps(t, withX(...))`), not a copy of the setup per test. Build only what the test
  needs; keep real filesystem and SQLite for integration tests (`t.TempDir()`). Every opened
  store is closed via `t.Cleanup` inside the fixture.
- **Helpers** call `t.Helper()` and fail with a concrete message (`got X, want Y`). No generic
  assertion DSL, no testify.

### Matrix (table-driven) tests

Use a table whenever several cases share the same setup and the same kind of assertion and
differ only in input and expected output: validation rules, parsers/serializers, slug and path
handling, error-code → status mapping, permission matrices (role × action), Unicode and path
edge cases.

```go
func TestSlugService_GenerateValidSlug_NormalizesTitles(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "spaces become dashes", input: "Hello World", want: "hello-world"},
		{name: "umlauts are transliterated", input: "Größe", want: "grosse"},
		{name: "punctuation is dropped", input: "C/C++ Guide", want: "c-c-guide"},
	}
	svc := NewSlugService()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := svc.GenerateValidSlug(tc.input); got != tc.want {
				t.Errorf("GenerateValidSlug(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
```

```ts
it.each([
  { name: 'plain text', template: 'page not found', args: [], want: 'page not found' },
  { name: 'fills %s in order', template: 'asset %s not found', args: ['a.png'], want: 'asset a.png not found' },
])('formatLocalizedErrorTemplate: $name', ({ template, args, want }) => {
  expect(formatLocalizedErrorTemplate(template, args)).toBe(want)
})
```

Every row has a `name` (Go) or a readable title (TS) that says what the case is about. Don't
force a table when cases need different setup or different assertions; then separate tests
are clearer. A table with `if tc.expectError` branches that do completely different checks is
a sign to split it.

### Other rules

- **Bugs get a regression test first:** reproduce the bug with a test, confirm it fails for
  the right reason, then fix it. The test stays as the guard.
- Names: `Test<Type>_<Scenario>_<Expectation>` in Go; `describe('<unit>')` + `it('<does what>
  when <condition>')` in TS.
- No mass snapshots; they pin implementation details. Assert the behaviour that matters.
- `act(...)` and i18n warnings in a test run are bugs in the test: await async effects, check
  expected error logs explicitly, don't silence them globally.
- Stores and module-level state are reset between tests.
- Tests run on fresh temporary data only, never on a real instance.

### Leave tests better than you found them

When a change touches a test file, tidy the tests **around the change** in the same PR:
merge copy-pasted cases into a table, extract repeated setup into the fixture, rename tests to
the convention, remove dead helpers. Constraints:

- **Behaviour-preserving:** no production code changes, no weaker assertions, **no lost case**.
  Every old case becomes a table row or stays a test. Coverage of the package must not drop
  (compare `go test -cover` / `npm run test:coverage` before and after).
- **Separate commit** (`test: tidy <area>`), so the reviewer can check it on its own.
- **Bounded:** only the test files the change already touches. Larger restructurings (splitting
  a whole file, a new shared fixture across packages) are their own PR via the `tidy-tests`
  skill.

## Pull requests

- **Size: at most 400–500 changed lines of production code** — hard limit. Not counted: tests
  (`*_test.go`, `*.test.ts(x)`, `e2e/`, `testdata/`, `fixtures/`), documentation (Markdown,
  ADRs, `docs/`), lockfiles, generated files, pure file moves/renames, and locale files other
  than `en/`. Tests don't count so that nobody saves on them, but they are reviewed like code.
- **Soft limit: ~1,000 changed lines in total** (production + tests). Above that, the PR
  description says why it can't be split.
  Larger work is split into a series of PRs that each leave `main` working.
- **`main` must always work.** A feature that needs several PRs goes behind a feature flag that
  defaults to off: a CLI flag + env var (`--enable-<feature>` / `LEAFWIKI_ENABLE_<FEATURE>`,
  marked experimental in `--help`), passed to the frontend via `GET /api/config` like
  `enableLinkRefactor`. The final PR of the series turns it on (or removes the flag) and adds
  the docs.
- **README changes go in their own PR**, merged when the release ships, so `README.md` on
  `main` never advertises an unreleased feature. Reference docs under `docs/` that belong to a
  change go in the same PR as the code.
- One topic per PR; refactoring and behaviour changes in separate PRs.
- One logical change per commit, in the [commit style](../../CONTRIBUTING.md#commit-style).
  Formatting is checked in CI (`npm run format:check`); run `npm run format` before committing.
- Commit subjects are checked by `.github/workflows/commit-messages.yml`
  (`make verify-commits BASE=origin/main` locally). Informational, not required: PRs are
  usually rebase-merged; one with messy commits is squash-merged under a title the maintainer
  sets, so the PR title only matters then.
