# ADR-0014: Generated, incrementally covered API documentation (Swagger 2.0 via pinned swag)

## Status

Proposed (2026-10-15)

## Context

LeafWiki's HTTP API has no machine-readable description. Automation users reverse-engineer
requests from the frontend, and the API is gaining non-browser callers now that API keys can
write ([#1662](https://github.com/perber/leafwiki/pull/1662)). The roadmap
([#1661](https://github.com/perber/leafwiki/issues/1661)) wants the API documented one package
at a time without a big-bang rewrite.

Constraints: the binary must stay free of documentation tooling (one binary for Linux, Windows,
macOS and Raspberry Pi, including linux/386); the existing routes, auth rules and error shapes
must not change; and the response shapes are inconsistent today (localized, flat and validation
errors), which the documentation must show instead of hiding.

## Decision

- **Generated, not handwritten.** The specification is generated from annotations in Go comments
  next to the handlers and from named response types, so code and documentation change in the
  same diff. Output is committed as `docs/api/swagger.json` and `docs/api/swagger.yaml` and is
  never edited by hand. A handwritten spec would be a second source of truth that drifts.
- **Swagger 2.0 with swag pinned at v1.16.6.** `make api-docs` runs
  `go run github.com/swaggo/swag/cmd/swag@v1.16.6 init …`. swag v2 (OpenAPI 3.x) is a different
  major line with its own migration cost; moving to it is a later, separate decision. The format's limits are accepted and
  documented: no `oneOf` (broad object schema for same-status alternatives such as the health
  503), no cookie security scheme (cookies are described in prose).
- **Dependency isolation.** The generator is not in the root `go.mod`, there is no `tool`
  directive, no `docs.go` and no swag import, so `go build` and `go test` never download or run
  it. Only the dedicated `api-docs` CI job executes it.
- **Incremental coverage with exact debt.** `internal/http/api_docs_test.go` compares the
  real, fully populated router (method plus path) with the committed JSON. Every undocumented
  operation is an individual entry in `api_docs_allowlist_test.go`; there are no wildcards or
  prefixes. Unknown routes, stale entries, entries for already documented operations and
  phantom documented operations fail the test. Each package PR removes the entries it covers.
  The entries are documentation debt, not an authorization allowlist.
- **CI enforcement.** The `api-docs` job in `backend.yml` runs `make api-docs`, fails on
  `git diff --exit-code -- docs/api` and uploads both files as the `api-docs` artifact.
- **Shared error models, no migration.** `internal/http/dto` gets `ErrorResponse`,
  `ErrorDetail` and `LegacyError`. Existing package helpers migrate in their own documentation
  PRs; incompatible variants are reported on #1661.
- **Required means always present.** swag runs with `--requiredByDefault`, so a field without
  `omitempty` is listed as `required` and a field with `omitempty` is optional. That is the
  same rule `encoding/json` uses to decide whether a key is written, so the `required` lists
  follow from the struct tags and cannot drift; `api_docs_test.go` checks each definition
  against its Go type. Required means the key is present, not that the value is non-null
  (Swagger 2.0 has no `nullable`), so mappers normalize nil slices and maps in required
  fields to empty ones.
- **No core types in documented responses.** Documented response types use `dto` types such
  as `dto.UserLabel` instead of `internal/core` types, so core types stay out of the public
  contract. `api_docs_test.go` rejects any definition whose Go type lives in
  `internal/core`.
- **Deferred.** Serving the specification or a viewer from the running server, swag v2/OpenAPI
  3, an API stability promise (`info.version` is `0.x`), and documenting the packages beyond
  health, tags, properties, search and links.

Contributor rules are in [docs/api/README.md](../../api/README.md).

## Alternatives considered

- **Handwritten OpenAPI file**: no coupling to the handlers, so it drifts silently; the
  route-coverage test could only check paths, never shapes.
- **swag as a module dependency or `tool` directive**: adds the generator and its transitive
  dependencies to the module graph, `go mod download` and the vulnerability scans for a tool
  the binary does not use. The compiled binary only contains packages it actually imports, so
  this is about footprint and review surface, not binary contents.
- **swag v2 / OpenAPI 3.x now**: would express `oneOf` and cookies better, but it is a
  larger migration than a skeleton warrants. Revisit when the documentation has grown enough
  for the limits of Swagger 2.0 to hurt.
- **Serve the spec (gin-swagger, embedded file)**: adds a runtime surface, dependencies and an
  exposure/auth question for a documentation set that is mostly empty. Deferred until coverage
  is meaningful.
- **Per-field `binding:"required"` or `validate:"required"` tags** for the required lists:
  `binding` turns on gin validation, which changes request behaviour; `validate` on every
  field works but is noisy and easy to forget. swag has no field-level comment for it.
- **Prefix or per-package coverage exceptions**: would silently excuse new routes. Exact
  entries force a conscious reviewed decision.
- **Normalize error responses first**: changes the wire format that the frontend and existing
  clients rely on; out of scope for a documentation PR.

## Consequences

- Adding or changing an API route requires documentation or a reviewed exception, and a
  regenerated spec; CI blocks stale output.
- Contributors need network access to fetch the pinned generator the first time they run
  `make api-docs`, but nobody else does.
- The coverage test proves inventory (method and path), not that annotations match handler
  behaviour; per-package shape tests close that gap.
- Swagger 2.0 cannot describe some real behaviour exactly (alternatives of one status, cookie
  auth); descriptions carry that, and the approximations are not authorization.
- Request and query structs document every field without `omitempty` as required. An
  optional request field needs `omitempty` or `validate:"optional"`; never
  `binding:"optional"`, which makes gin panic.
- A field without `omitempty` that can be `null` is documented as required although clients
  may see `null`: `dto.Node.Children` is `null` for leaf nodes and must be normalized (or
  noted) when pages are documented.
- Undocumented responses still expose core types, to be replaced when their packages are
  documented: `dto.Node.Kind` and `pages.WikiLinkMatch.Kind` (`tree.NodeKind`).
- Revisit when Swagger 2.0's limits block accurate documentation, when most routes are documented (serving the spec), or when
  the API gets a stability guarantee.
