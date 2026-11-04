# ADR-0018: The MCP server depends on mcp-go and plugs into the RouteRegistrar seam

## Status

Proposed (2026-11-02)

## Context

LeafWiki wants an MCP (Model Context Protocol) server so AI agents can read
and edit pages over one HTTP endpoint instead of a per-feature REST surface.
The MCP streamable-HTTP transport is a JSON-RPC protocol: an `initialize`
handshake that issues a session ID, JSON-RPC messages POSTed to a single
endpoint, an optional SSE notification stream, and per-session state.

Writing that layer by hand is not a ~50-lines job: the spec covers protocol
version negotiation, session lifecycle (create, reuse, terminate, validate),
JSON-RPC framing, and the SSE/JSON response switch, and the spec keeps
evolving. `github.com/mark3labs/mcp-go` (v0.55.0) is the maintained Go
implementation of exactly that protocol layer. What it replaces:

- the JSON-RPC 2.0 framing and MCP method handling (initialize, tools/list,
  tools/call, …) — pure protocol, no LeafWiki logic;
- the streamable-HTTP transport (session ID generation/validation, SSE
  upgrade, GET notification stream, DELETE session teardown).

Weight: pure Go, no cgo, all target platforms (Linux amd64/arm64/386,
Windows, macOS, Raspberry Pi) build as before. It grows the single binary by
roughly 1–2 MB (the `server` and `mcp` packages plus a small transitive tree:
`google/uuid`, `google/jsonschema-go`, `santhosh-tekuri/jsonschema`,
`spf13/cast`, `yosida95/uritemplate`). Licence MIT, actively maintained.

The endpoint needs the caller's identity for the tool calls: the same trust
question as the REST API — who is this, and did they come through a browser
session or a Bearer API key?

## Decision

1. **Protocol layer:** `github.com/mark3labs/mcp-go` v0.55.0 (MIT, pure Go,
   no cgo). The slice uses `server.NewMCPServer` and
   `server.NewStreamableHTTPServer` as an `http.Handler`; `server.
   WithHTTPContextFunc` is the one hook that carries request-scoped values
   into the MCP request context the tools receive. The streamable server runs
   with `server.WithStateful(true)` (session IDs are tracked; the
   transport's DELETE verb really terminates a session, unknown IDs get a
   404 — mcp-go's default manager issues IDs but never validates them) and
   `server.WithSessionIdleTTL(30*time.Minute)` (idle sessions are reaped
   even when the client never sends the teardown DELETE).
2. **Registration:** a new slice `internal/wiki/mcp` with the standard
   `Routes`/`RoutesConfig` shape, registered through the existing
   `RouteRegistrar` seam — conditionally, behind `--enable-mcp`
   / `LEAFWIKI_ENABLE_MCP` (default off, experimental), the same way
   `backupRoutes`/`snapshotRoutes` are appended. No `if feature` branches in
   shared code. The endpoint is `POST|GET|DELETE <base>/api/mcp` on
   `RouterContext.APIAuthGroup(authService)`, i.e. behind the existing
   session/API-key middleware chain, so an unauthenticated caller never
   reaches the protocol layer.
3. **Auth context:** the gin wrapper lifts the resolved user
   (`c.Get("user")`) and the key-auth marker (`IsAPIKeyAuth`) into the
   request context; `WithHTTPContextFunc` moves them into the MCP context.
   Tool handlers read them via `userFromContext`/`keyAuthFromContext`.
4. **Tool wiring (from the second PR on):** the slice's tool handlers are
   thin and delegate through a composition-root adapter
   (`WikiMCPAdapter`) to the existing pages/revisions/search use cases — the
   slice never talks to `internal/core` or the top-level index packages
   directly.
5. **Auth policy:** the endpoint uses the existing API-key/session
   middleware (a Bearer API key authenticates when API-key management is
   enabled), so a caller's identity and role are exactly what the REST
   API would see for the same credential.
6. **First PR scope:** endpoint skeleton only — a client gets a valid
   `initialize` and an empty `tools/list`. Zero tools registered.

### The page tools (PRs 2–3 of the staged rollout)

The second PR registered the four page-read tools (`get_page`,
`get_page_by_path`, `search`, `list_tree`, gated at `viewer`). The third is a
three-part series — `create_page` and `update_page` first, then `patch_page`,
then `delete_page` and `move_page` — each gated at `editor`, the role the
equivalent REST mutation requires, each delegating through the same use case
constructors the REST routes use (so `pagesave` events, `AffectedPages`, and
the asset/favorite/revision cleanup fire exactly as through HTTP) and mapping
the write-path tree sentinels onto the same `page_*` codes
`respondWithPageError` returns — through the non-HTTP mapper the route handler
now shares with every consumer (`pages.MapPageError`), so the two tables can't
diverge. One parity detail: an unknown create parent surfaces with
`page_not_found`, not `page_parent_not_found` — the REST create route answers
`page_not_found` because the create use case's parent check passes the raw
tree error through, and the write tools mirror that. Two consequences of the
write tools:

- **Body cap:** the transport's body cap grew from 1 MiB to
  `maxRequestBodyBytes` (500 MiB, a named constant in `routes.go`). The
  write tools carry full page content, so the cap has to accept the largest
  page payload the platform accepts anywhere — its own bulk-content uploads:
  the importer's ZIP upload cap (`importMaxUploadSize` in
  `internal/wiki/importer`) and the backup-restore upload default
  (`--restore-upload-max-size`, 500 MiB). The adjacent REST page-update route
  sets no cap at all, so the MCP cap is strictly tighter; looser would only
  widen an authenticated-DoS lever (mcp-go buffers and JSON-decodes the whole
  body per request).
- **Error logging:** a write failure that is neither a known domain error nor
  validation arrives at the `toolResult` choke point as `page_internal_error`
  — the 500-equivalent `LocalizedError` the REST route would return (a wrapped
  storage failure lands in the shared mapper's default arm). The choke point
  logs it exactly once, the way it logs `mcp_internal_error`: a failed tool
  answer is HTTP 200 and mcp-go never logs it, so without the line the
  storage failure would leave no trace at all — unlike the REST API, where
  the same failure is a visible 500. The match is on the `_internal_error`
  suffix every slice's 500-equivalent code carries by the `<domain>_<reason>`
  convention, so exposing more slices' tools later needs no change there.

### The tree tools (PR 4 of the staged rollout)

The fourth PR adds the three tree tools — `ensure_path`, `convert_node`,
`copy_page` — each gated at `editor`, the role the equivalent REST mutation
requires, and each a thin delegation to the existing use case the REST route
uses: `EnsurePathUseCase` (missing segments become sections, existing nodes are
reused — a repeated call creates nothing and never clobbers a title),
`ConvertPageUseCase` (the version comes from the live page; a section with
children cannot become a page, the REST-parity `page_has_children` — the
mapper's `page_convert_not_allowed` arm covers the node store's not-empty-folder
refusal, reachable only from an on-disk drift state), and `CopyPageUseCase`
(the REST single-page copy: the page and its assets, the children stay with
the source; a destination slug collision answers `page_slug_conflict`).

### The revision tools (PR 5 of the staged rollout)

The fifth PR adds the four revision tools: `get_page_revisions` (newest
first, paged with the REST route's 50/200 default/cap), `get_page_revision`
(metadata plus the content with line numbers and the revision's assets),
`compare_page_revisions` (the comparison shape — the content-change flag, the
asset differences, both contents — no diff algorithm) and
`restore_page_revision`. The three read tools are gated at `viewer`, the
minimum role the REST revision read routes sit behind (the plain API auth
group), and restore at `editor` — the `RequireEditorOrAdmin` middleware the
REST restore route carries. Restore goes through the same
`RestoreRevisionUseCase` the REST route uses, so the `pagesave` restore event
and the page-save metrics fire exactly as through HTTP — and the restore
itself is recorded as a new revision of type `restore`, so a rollback is
visible in the history like any other save. The adapter delegates the
revisions use-case errors to the slice's shared `MapRevisionError` mapper —
the non-HTTP core of `respondWithRevisionError`, so the REST route and the
MCP tools can never drift apart (a LocalizedError passes through, a store
`os.ErrNotExist` becomes `revision_not_found`, anything else
`revision_internal_error`) — and tools/list now carries all sixteen tools.

## Alternatives considered

- **Hand-rolled JSON-RPC/MCP layer:** ~50 lines gets you a request/response
  loop, not the session model, protocol-version negotiation and SSE upgrade;
  and the MCP spec is moving, so LeafWiki would re-implement the same fixes
  mcp-go ships. Rejected.
- **A separate MCP gateway process in front of the REST API:** a second
  binary/auth surface to run and secure, and the agent would hold REST
  credentials instead of going through LeafWiki's own middleware. Rejected;
  revisit if the protocol library ever becomes a liability.
- **Registering the endpoint directly in the central router:** violates
  Open/Closed — the `RouteRegistrar` seam exists precisely so slices register
  themselves without the router growing a branch. Rejected.
- **Stateless streamable-HTTP transport (`WithStateLess`):** would drop
  session IDs, but then per-session state (future: server-sent tool
  notifications, per-client tool sets) would not be available without a
  rewrite. Stateful mode (`WithStateful`) is the spec-conformant choice; its
  in-memory session store is a revisit trigger, not a blocker.

## Consequences

- A new runtime dependency with lasting weight: ~1–2 MB of binary on every
  platform, one more module to keep current (Trivy-scanned like the rest).
- The streamable server keeps per-session state in memory; a client that
  never sends the explicit teardown DELETE would otherwise leak it until
  process restart. The server is constructed with
  `WithSessionIdleTTL(30*time.Minute)` (server.go), so a background sweeper
  reclaims sessions idle longer than 30 minutes — before any real tool (or
  client) can accumulate meaningful per-session state.
- The binary grows even when `--enable-mcp` is off (Go does not split
  packages out at link time); the feature itself stays off by default.
- Tool handlers must treat the caller from the request context as the
  authoritative identity and re-check roles per tool.
- Revisit this decision if: the MCP spec's session model outgrows mcp-go's
  in-memory store (e.g. we need sessions to survive restarts or span
  processes), or a security/quality problem forces a switch of protocol
  library — whichever replacement lands must still register through the
  `RouteRegistrar` seam and authenticate through `APIAuthGroup` (the seam
  and the auth policy are independent of the protocol library).
