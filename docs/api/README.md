# LeafWiki API documentation (experimental)

[`swagger.json`](./swagger.json) and [`swagger.yaml`](./swagger.yaml) describe LeafWiki's HTTP
API as **Swagger 2.0**, version `0.x`. They are **experimental and incomplete**: so far only
`GET /api/health` and the tag and property reads (`/api/tags`, `/api/properties`) are
documented, and anything may change before v1. Discussion, inconsistencies
and deferred decisions are tracked in [#1661](https://github.com/perber/leafwiki/issues/1661).

The files are generated from Go comments and committed. Never edit them by hand. There is no
runtime documentation endpoint, Swagger UI or `docs.go`; the specs are plain files (CI uploads
them as the `api-docs` artifact).

## Generating

From the repository root, either

```text
make api-docs
```

or, without Make (Windows, for example), the same command directly:

```text
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --generalInfo api_docs.go --dir ./cmd/leafwiki,./internal --parseInternal --output ./docs/api --outputTypes json,yaml
```

The generator is pinned to swag v1.16.6 (Swagger 2.0; swag v2 would produce OpenAPI 3.x) and
runs through `go run`, so it is not in `go.mod` and ordinary builds and tests never need it.
The Makefile target is the single definition of version and flags. CI runs it and fails if
`git diff -- docs/api` is not empty, so commit regenerated files together with the annotation
change.

General metadata (title, version, the `BearerAuth` scheme, prose about cookies and CSRF) lives in
[`cmd/leafwiki/api_docs.go`](../../cmd/leafwiki/api_docs.go). Operation annotations sit directly
above their handlers, for example `handleHealth` in
[`internal/wiki/health/routes.go`](../../internal/wiki/health/routes.go).

## Conventions for documenting a package

- **Paths** are relative to `basePath: /api`: `@Router /pages/{id} [get]`. Use `{id}` for Gin
  `:id` and `{name}` for `*name`. The spec describes the default mount; a deployment started
  with `--base-path=/wiki` serves every documented URL under `/wiki`.
- **Tags**: one tag per slice directory (domain), lower case: `health`, `pages`, `backup` (not
  the Go package name `wikibackup`).
- **Models**: use named, package-local request/response types with explicit `json` tags, next
  to the slice (see `internal/wiki/health/api_types.go`). Reuse shared types from
  `internal/http/dto` where they exist. Do not use `gin.H` or anonymous structs for documented
  bodies. Replacing `gin.H` with a named type must not change the wire format; add a test.
- **Errors**: see below. Document a failure status only if a middleware or handler on that
  route can actually emit it.
- Keep summaries short and say what is true for the route today, including configuration-
  dependent behaviour (for example anonymous reads when public access is on).

### Authentication in the annotations

There is deliberately **no global security requirement**: an omitted requirement does not
mean a route is public, so each operation states its access in its description.

- **Bearer API key** (`BearerAuth`, Swagger 2.0 `apiKey` in header `Authorization`): send the
  *entire* value `Bearer lw_<prefix>_<secret>`, not just the secret. Keys only exist when
  LeafWiki runs with `--enable-api-key-management` and authentication enabled. A key acts with
  the lesser of its owner's role and its own role, so role guards (`RequireAdmin`,
  `RequireEditorOrAdmin`) still apply. Annotate routes that accept keys with
  `@Security BearerAuth`.
- **Cookie session** (browser): the access and refresh cookies are `__Host-leafwiki_at` and
  `__Host-leafwiki_rt` on secure connections and `leafwiki_at` / `leafwiki_rt` where insecure
  development is allowed. Swagger 2.0 cannot express cookies as an alternative to a header
  scheme, so cookie access is written in the description, not as a security definition.
  Reverse-proxy header auth is another alternative front door; describe it in prose when it
  matters.
- **CSRF**: writes (anything but GET/HEAD/OPTIONS) behind the standard auth groups need the
  CSRF cookie (`__Host-leafwiki_csrf`, or `leafwiki_csrf` for insecure development) **and** a
  matching `X-CSRF-Token` header; the `csrf_token` form field is accepted as a fallback.
  Login, TOTP login and refresh-token are *not* behind the CSRF middleware, and neither are
  other pre-auth routes: describe the credentials or cookie they actually use. A request
  authenticated by a *resolved* API key skips CSRF; a merely present `Authorization` header
  does not.
- **Session-only routes**: routes guarded by `RequireCookieSession` reject API keys with 403.
  Examples: API-key management, creating/updating/deleting users, and snapshot download.
  Not every admin route is session-only (for example `GET /api/users` accepts keys), so check
  the route registration (`grep RequireCookieSession`) instead of assuming. For session-only
  routes omit `@Security BearerAuth` and write "browser-session only" plus the role
  requirement in the description.

### Error shapes

The server emits three different error bodies today. Do not invent a cleaner one; pick the
model that matches the code path, and report inconsistencies on #1661.

| Shape | JSON | Model |
|---|---|---|
| Localized | `{"error": {"code", "message", "template", "args"?}}` | `dto.ErrorResponse` / `dto.ErrorDetail` |
| Flat (middleware and older handlers) | `{"error": "message"}` | `dto.LegacyError` |
| Validation | `{"error": "validation_error", "fields": [...]}` | none yet (package-local; do not extend `ErrorDetail`) |

Package helpers (for example in `internal/wiki/pages` and `internal/wiki/search`) still declare
their own copies of the localized shape; migrating them to `dto` happens in each package's
documentation PR with a wire-compatibility test.

Standard failures, and where they apply:

| Status | Emitted by | Shape | Applies to |
|---|---|---|---|
| 401 | `InjectAPIKeyUser`: invalid, revoked or expired key | flat | every route, only when key management is enabled and a key-shaped Bearer token reaches key resolution (no `Authorization` header, a non-key Bearer value, or an identity already resolved upstream, such as by reverse-proxy auth, bypasses it) |
| 401 | `RequireAuth`: missing or invalid access token | flat | routes behind the auth groups |
| 401 | reverse-proxy auth: unknown user | flat | every route, only when reverse-proxy auth is enabled |
| 403 | `CSRFMiddleware`: missing or mismatching token | flat | methods other than GET, HEAD and OPTIONS on routes behind the CSRF middleware (not login/refresh); skipped for requests authenticated by a resolved API key |
| 403 | `RequireCookieSession`, `RequireAdmin`, `RequireEditorOrAdmin`, `RequireSelf` | flat | only routes carrying that guard |
| 429 | `InjectAPIKeyUser`: too many failed key attempts | flat | every route, only when key management is enabled and a key-shaped Bearer token reaches the rate limiter (same bypasses as the 401 row) |
| 503 | `InjectAPIKeyUser`: key store suspended during a live restore | flat | every route, only when key management is enabled and a key-shaped Bearer token reaches key resolution (same bypasses as the 401 row) |
| 503 | write gate while a live restore runs | localized | methods other than GET, HEAD and OPTIONS, except the restore endpoints and login (including TOTP login), logout and refresh-token; never GET routes such as health |

Health does not require authentication and never validates session cookies, so it documents 401
and 429 only as consequences of global middleware: an invalid, revoked or expired *supplied*
API key (key management enabled), a trusted-proxy identity that LeafWiki rejects (unknown user
or auto-provisioning email conflict, reverse-proxy auth enabled), or throttled key attempts. It
documents no 403.

### Same-status alternatives (the 503 limitation)

Swagger 2.0 has no `oneOf`. `GET /api/health` can return 503 with two shapes: a degraded
`HealthResponse` (`status: "degraded"`, failed `checks`), or a flat `LegacyError` when API-key
resolution cannot reach its store. The skeleton approves an intentionally broad `object` schema
for that 503 and names both shapes in the response description, instead of a union type, an
invented normalized body or a changed middleware response. Reuse this pattern whenever one
status has several real shapes, and note it on #1661.

## Route coverage and exceptions

`internal/http/api_docs_test.go` builds the real router with every registrar (including the
separately attached backup, snapshot, restore and instance-settings routes) and compares
**method plus path** with `docs/api/swagger.json`. Operations that are not documented yet are
listed one by one in `internal/http/api_docs_allowlist_test.go` as
`"METHOD /api/path": "<package>: <reason>"`. This is documentation debt, not an authorization
allowlist, and there are no wildcard or prefix entries. The test fails when:

1. a real route is neither documented nor listed,
2. a listed route does not exist (stale entry),
3. a listed route is also documented,
4. a documented operation has no real route.

When documenting a package:

1. Annotate its handlers and add or reuse named models.
2. Run `make api-docs` and commit the regenerated files.
3. Delete exactly the exception entries for the operations you documented (the test fails
   until you do), and add any new undocumented route as its own reviewed entry only if it
   cannot be documented in the same PR.
4. Add shape tests for the package, like the health tests: assert status codes and the exact
   JSON keys and types of the real responses. The coverage test only proves that a route is
   listed, not that the annotation matches the handler.
5. Do not migrate error helpers or extract models of other packages in the same PR.
