# ADR-0010: Opt-in auto-provisioning for reverse-proxy header auth

## Status

Accepted (2026-07-31).

## Context

[ADR-0005: Reverse-Proxy Header Auth](./0005-reverse-proxy-header-auth.md) added `InjectRemoteUser` to trust an identity asserted by a reverse proxy (Authelia, Authentik, oauth2-proxy, Tailscale-based setups) instead of requiring a separate LeafWiki login. It deliberately scoped out auto-provisioning: "the proxy-asserted username must already exist as a LeafWiki user."

[Issue #1330](https://github.com/perber/leafwiki/issues/1330) asks for that scope limit to be lifted: operators running LeafWiki behind oauth2-proxy don't want to manually pre-create a LeafWiki account for every identity the proxy already authenticates — every proxy-authenticated identity should just work, with a configurable default role since the proxy, not LeafWiki, owns authentication.

The risk ADR-0005 called out — a forged header (from a misconfigured trusted-proxy allowlist, or LeafWiki becoming reachable directly) impersonates a user — gets strictly worse with auto-provisioning: a forged header could also mint a brand-new account. This ADR keeps that widened blast radius bounded rather than accepting it unconditionally.

## Decision

Add `UserService.GetOrCreateRemoteUser` (`internal/core/auth/user_service.go`) and wire it into `InjectRemoteUser` (`internal/http/middleware/auth/reverse_proxy.go`) behind a **separate** opt-in flag, `--enable-http-remote-user-auto-create` / `LEAFWIKI_ENABLE_HTTP_REMOTE_USER_AUTO_CREATE`, distinct from and dependent on `--enable-http-remote-user`. Auto-create is off by default, so ADR-0005's original behavior (unknown proxy-asserted users get a 401) is unchanged unless an operator explicitly turns it on.

When enabled:
- An identifier with no matching user (looked up the same way as an existing user, via `GetUserByIdentifier`: username first, then email) is created as a new user rather than rejected.
- The identifier becomes the new account's `username` verbatim. An optional second header (`--http-remote-user-email-header-name`) supplies the email; if absent, a non-deliverable placeholder is synthesized under the RFC 2606 `.invalid` reserved TLD.
- The new account gets a cryptographically random password it can never be told (same pattern already used by `ResetAdminUserPassword` for admin-recovery accounts) — it can only ever authenticate via the trusted proxy, never the built-in login form.
- The assigned role comes from `--http-remote-user-default-role` (default `viewer`), which the CLI startup validation (`validateHTTPRemoteUserAutoCreateConfig`) refuses to let be `admin`. A forged or misrouted header must not be able to mint an admin account by itself; an operator who wants a remote-provisioned admin can promote the account manually after it's created.
- A race between two concurrent requests for the same brand-new identifier is resolved by re-fetching on `ErrUserAlreadyExists` rather than double-creating (the existing `UserStore` mutex already serializes the underlying store writes).

## Consequences

- Self-hosters using oauth2-proxy/Authelia-style setups no longer need to pre-provision every user by hand — the common case ADR-0005 left as manual toil is now automatic, opt-in.
- Security still depends entirely on the trusted-proxy-IP configuration being correct, same as ADR-0005; auto-create widens what a forged header can do (create new accounts, not just impersonate existing ones), which is why it requires its own explicit flag rather than being implied by `--enable-http-remote-user`, and why the default role can never be `admin`.
- ADR-0005's own text is left as historical record (it was accurate when written); this ADR supersedes only the "can't auto-provision" limitation it documented, not the rest of its reasoning.
- Auto-created usernames are whatever the proxy's primary header asserts, even if that value looks like an email address — no heuristic is applied. Operators whose proxy sends an email in the primary header should point the (optional) email header at the same or another proxy header if they want a distinct, human-readable username.
