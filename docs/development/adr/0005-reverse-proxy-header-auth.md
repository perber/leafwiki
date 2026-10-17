# ADR-0005: Optional reverse-proxy header auth as an alternate front door, not a second auth system

## Status

Accepted (retroactively documented, 2026-07-07).

## Context

Some self-hosters already run an SSO reverse proxy in front of everything on their network (e.g. Authelia, Authentik, Tailscale-based setups) and want LeafWiki to trust that proxy's identity assertion rather than maintaining a separate LeafWiki login. Building full OIDC/SAML support into LeafWiki itself would be a significant addition to a project whose stated principle is "minimal operational complexity," and would duplicate identity management the self-hoster already has.


## Decision

Add `InjectRemoteUser` middleware (`internal/http/middleware/auth/reverse_proxy.go`) that, only when explicitly configured with a trusted header name and a trusted-proxy IP allowlist, reads the identity from that header, looks the user up via the existing `UserService.GetUserByUsername`, and short-circuits normal JWT validation for that request. It reuses `internal/core/auth`'s existing `UserService`/roles rather than introducing a parallel identity or session model.

## Consequences

- Self-hosters get SSO integration for free, without LeafWiki needing to speak any federation protocol.
- Security depends entirely on the trusted-proxy-IP configuration being correct: if LeafWiki is ever reachable directly (bypassing the proxy), an attacker can forge the trusted header and impersonate any user. This is a sharp edge that must be called out explicitly in setup docs and in any code review touching this middleware or the deployment/networking docs.
- Because it reuses `UserService`, this feature can't be used to auto-provision unknown users — the proxy-asserted username must already exist as a LeafWiki user. That's a deliberate scope limit, not an oversight.
