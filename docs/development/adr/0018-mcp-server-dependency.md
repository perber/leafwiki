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
