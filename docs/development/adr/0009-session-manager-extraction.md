# ADR-0009: Extract SessionManager from AuthService as the seam for future non-password identity providers

## Status

Accepted (2026-07-23).

## Context

`internal/core/auth.AuthService` had grown into a single struct handling three
distinct concerns: local password verification (`Login`), TOTP two-factor
verification (`CompleteTOTPLogin`, TOTP self-service), and JWT/session
management (issuing, refreshing, revoking, and validating access/refresh
tokens). There was no boundary between "who is this user" and "issue/manage a
session for this user" — both lived inline in the same 778-line file.

Separately, there is a confirmed intent to add native OIDC support to LeafWiki
in the future (LeafWiki itself speaking to an OIDC provider, not just trusting
a header from a reverse proxy). This is a deliberate addition alongside — not
a replacement for — the proxy-header trust model in
[ADR-0005: Reverse-Proxy Header Auth](./0005-reverse-proxy-header-auth.md): that ADR's reasoning
(avoid duplicating identity management self-hosters already run in front of
LeafWiki) still holds for that deployment shape, but it does not preclude
LeafWiki also supporting native OIDC for setups without such a proxy.
[ADR-0005: Reverse-Proxy Header Auth](./0005-reverse-proxy-header-auth.md) is not superseded by this decision.

Any future non-password identity provider (OIDC or otherwise) needs to end at
the same place local password login does once a user's identity is
established: "issue a LeafWiki session (JWT access+refresh tokens) for this
`*User`." That shared endpoint did not exist as its own reusable unit — it was
inline logic private to `AuthService`.

## Decision

Extract the session/token logic out of `AuthService` into a new, standalone
`SessionManager` (`internal/core/auth/session_manager.go`), covering:
`IssueSession`, `RefreshToken`, `RevokeRefreshToken`, `RevokeAllUserSessions`,
`RevokeAllUserSessionsExceptCurrent`, `ValidateToken`, and the underlying
JWT sign/parse plumbing.

`AuthService` keeps password verification and TOTP handling, and holds a
`*SessionManager` it delegates to. Its public method set is unchanged — every
existing caller (`internal/wiki/auth` use cases, `internal/http/middleware/auth`)
keeps calling `AuthService.RefreshToken`/`ValidateToken`/etc. exactly as
before; only the internal implementation moved.

No new interface (e.g. `IdentityProvider`) was introduced. There is exactly
one caller of `SessionManager` today (`AuthService`); a speculative interface
for a second, not-yet-built caller would be a premature abstraction. A future
OIDC service can depend on the same concrete `*SessionManager` type directly —
the composition root (`internal/wiki/wiki.go`) already builds one
`SessionManager` per instance and would hand the same pointer to a future
`OIDCService` alongside `AuthService`, rather than each provider building its
own session handling.

`SessionManager.resolveUser` (a `func(id string) (*User, error)`) is wired by
`NewAuthService` to `a.users` — the same mutex-guarded accessor
`ReplaceUserStore` swaps during a live restore — rather than to a fixed
`*UserService` captured at construction time. This preserves the existing
hot-swap guarantee: `RefreshToken`/`ValidateToken` always resolve against
whatever `UserService` is current, not the one at startup.

## Consequences

- `AuthService` is smaller and its remaining code is exclusively about
  password+TOTP verification; session/token mechanics are independently
  testable via `session_manager_test.go` without going through password
  login.
- A future OIDC callback handler can call `sessionManager.IssueSession(user)`
  directly once it has resolved/provisioned a `*User`, reusing token
  issuance, refresh, and revocation as-is. Whether OIDC users are
  auto-provisioned or must already exist as LeafWiki users (as
  [ADR-0005: Reverse-Proxy Header Auth](./0005-reverse-proxy-header-auth.md) deliberately requires for the
  proxy-header case) is not decided by this
  ADR — that is scoped to the future OIDC implementation itself.
- This was a behavior-preserving internal refactor: no HTTP contract, config
  flag, or frontend behavior changed. The full existing test suite passed
  unmodified in behavior (only construction call sites were mechanically
  updated for the new, smaller `NewAuthService`/new `NewSessionManager`
  constructor signatures).
