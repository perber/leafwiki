# ADR-0014: Native OpenID Connect login as an optional, CLI/ENV-configured feature

## Status

Proposed (2026-10-19)

## Context

LeafWiki's only built-in login is username/email + password, optionally with TOTP. Single
sign-on is possible today only through a reverse proxy that authenticates the user and
asserts the identity in a header ([ADR-0005](./0005-reverse-proxy-header-auth.md),
auto-provisioning in [ADR-0010](./0010-remote-user-auto-provisioning.md)). That works, but it
means running and securing an extra component (oauth2-proxy, Authelia, …) in front of every
instance, keeping the trusted-proxy allowlist correct, and accepting that a forged header is
enough to impersonate a user.

Operators who already run an OpenID provider (Keycloak, Authentik, Zitadel, Entra ID, Google
Workspace, Dex, …) want LeafWiki to talk to it directly, and organisations running several
LeafWiki instances want one login across all of them, with access and roles managed in one
place.

Constraints:

- Minimal operational complexity: an instance without OIDC configuration must behave exactly
  as before.
- One binary for Linux, Windows, macOS and Raspberry Pi: no cgo, no external helper process.
- The identity provider may start after LeafWiki or be temporarily unreachable (home labs,
  small servers that boot everything at once).
- [ADR-0009](./0009-session-manager-extraction.md) already made `SessionManager` the seam for
  other identity providers: anything that has identified a `*User` calls `IssueSession`.
- Optional server-level features are configured by CLI flags / ENV vars only
  ([ADR-0011](./0011-smtp-cli-env-optional-feature.md)).

## Decision

**Protocol.** LeafWiki acts as an OpenID Connect relying party for exactly one provider,
using the Authorization Code flow with PKCE (S256), a confidential client (client secret),
`state` and `nonce`. Libraries: `github.com/coreos/go-oidc/v3` and `golang.org/x/oauth2`
(pure Go). ID tokens are verified for signature (provider JWKS), `iss`, `aud`, `exp` and
`nonce`. Provider discovery happens lazily on first use and is retried; an unreachable
provider never blocks startup.

**Flow.** `GET /api/auth/oidc/login` stores `state`, `nonce`, the PKCE verifier and the
requested return path in a short-lived, signed, HttpOnly, `SameSite=Lax` cookie (the callback
is a cross-site top-level navigation) and redirects to the provider.
`GET /api/auth/oidc/callback` checks the state cookie, exchanges the code, verifies the ID
token, resolves the user (below) and issues a normal LeafWiki session through
`SessionManager.IssueSession`, setting the same cookies as the password login. The return path
is accepted only as a same-origin relative path. Failures redirect to the login page with an
error code that the frontend translates. No server-side state store is added.

**Identity.** A new table `user_identities (issuer, subject, user_id, provisioned,
created_at)` in `users.db`, owned by `UserStore` (same handle, migrations and corruption
policy, so live restore and hot-swap cover it), links a provider identity to a LeafWiki user. The key is always
`issuer` + `sub`; username and email are never used as the identity, because both can change
at the provider. On login:

1. A known identity resolves to its user; the user's email is updated if the provider marks
   it verified.
2. An unknown identity may be linked to an existing user with the same email only if
   `--oidc-link-by-email` is set, the provider sends `email_verified=true`, and exactly one
   user has that email.
3. Otherwise, with `--oidc-auto-create`, a new user is created (username from
   `--oidc-username-claim`, made unique), sharing the provisioning code with
   `GetOrCreateRemoteUser` from ADR-0010.
4. Otherwise the login is refused.

Deleting a user deletes its identities in the same transaction (`users.db` does not enable
foreign keys, so this is an explicit delete, not `ON DELETE CASCADE`). `provisioned` is audit
data only; no login path reads it.

**Access and roles from claims (optional).** `--oidc-groups-claim` names a string-list claim.
`--oidc-allowed-groups` restricts login to members of at least one listed group and is checked
on every OIDC login, also for linked users. If `--oidc-admin-groups` or
`--oidc-editor-groups` is set, the provider becomes the source of truth for the role of OIDC
users: the role is recomputed on every OIDC login (`viewer` if no group matches) through a
`core/auth` method with a defined contract, and the user API refuses role changes for those
users (see "Where users are managed"). If the computed role would exceed the editor limit
(`--editor-limit`), the user gets `viewer` and a warning is logged, instead of refusing the
login. Unlike the proxy header
(ADR-0010), a claim may grant `admin`, because it comes from a signed token of the configured
issuer; it still has to be configured explicitly. Without role groups, roles stay managed in
LeafWiki and auto-created users get `--oidc-default-role` (default `viewer`, `admin` refused).

**Where users are managed.** The user list always stays in LeafWiki: authorship,
favorites, user settings, API keys and local accounts are attached to it. What the
configuration decides is who controls access and roles:

- *Roles in LeafWiki* (no role groups configured): the provider only authenticates. Admins
  assign roles in LeafWiki as today and can pre-create a user with the right role and email,
  which `--oidc-link-by-email` links on the first OIDC login. `--oidc-allowed-groups` can
  still restrict who gets in.
- *Roles at the provider* (`--oidc-admin-groups` or `--oidc-editor-groups` set): access and
  role of OIDC users come from the provider; LeafWiki shows the role read-only.

In both modes, local password accounts can exist next to OIDC users and are managed in
LeafWiki.

The user admin does not learn about OIDC. A new extension point, a *user account policy*
registered by the composition root, tells the auth slice per user which fields are managed
elsewhere (`managedFields`, e.g. `role`) and which annotations to show (`annotations`: kind,
label, optional URL and time, e.g. "single sign-on", "last OIDC login", "manage members"
linking to `--oidc-user-management-url`). `UpdateUserUseCase` refuses changes to managed
fields with the generic code `user_field_managed`; the user list returns both lists, and the
frontend renders managed fields read-only and annotations as generic badges and links. The
existing `--user-management-url`, which replaces the whole user area with a link, keeps its
meaning.

**User admission and local credentials.** Two generic user states in `core/auth`, independent
of OIDC:

- `deactivated`: the user cannot get in by any method, their sessions are revoked, their
  content and history stay. The last admin cannot be deactivated. Deleting an OIDC user is
  not a way to keep them out (with `--oidc-auto-create` the next login recreates the
  account); deactivating is. In *roles at the provider* mode, deactivation in LeafWiki blocks
  in addition to the provider, never the other way round.
- *no local password*: password login, password reset and invite acceptance are refused.
  `UserService` enforces it where passwords are verified or set. Accounts auto-created by
  OIDC or by reverse-proxy auth (ADR-0010) get this state, so "forgot password" cannot turn
  them into password accounts. Existing proxy-provisioned accounts are not migrated.

A single admission check in `core/auth` (`user_deactivated`, and the API key rule below) runs
at the three points every identity passes: `SessionManager.IssueSession` (password, TOTP,
invite, OIDC), the user resolution behind `ValidateToken`/`RefreshToken`, and the request
middleware that accepts users injected upstream (reverse-proxy header, API key), including
the optional-auth path. No login path adds its own check.

**Password login next to OIDC.** Users created by OIDC have no local password (above), so
the provider and its MFA cannot be bypassed. Linked users keep their password.
`--disable-password-login` (default off, general, not OIDC-specific) turns off password login
for everyone except admins, so that `leafwiki reset-admin-password` remains a way back in. The
composition root passes it to the password slice as a policy and leaves `password` out of the
announced login methods. LeafWiki's TOTP is not requested on OIDC
logins; MFA belongs to the provider.

**Sessions.** By default, the LeafWiki session is independent of the provider session after
login. Logout ends the LeafWiki session only.

**Re-checking with the provider on session refresh (optional).** With
`--oidc-refresh-with-provider` (default off), the callback also stores the provider's refresh
token, encrypted at rest (AES-GCM, key from `--oidc-token-encryption-key`, at least 32 bytes;
the encryption helper is extracted from `TOTPService` and shared).

This needs two changes to the session layer in `core/auth`, made as general seams before
OIDC uses them:

- Sessions get a stable *session ID* (a family ID that survives refresh-token rotation),
  returned by `IssueSession` and `RefreshToken`.
- `SessionManager.RefreshToken` calls registered *refresh checks* with user ID and session ID
  before it mints new tokens; a check can let the refresh continue or revoke the session.
  Password sessions have no check registered.

Provider tokens are stored in `sessions.db`, keyed by session ID and owned by `SessionStore`,
so every way a session ends (logout, revoke, revoke-all, cleanup of expired sessions, the
reset during a restore) removes them too, and they never end up in snapshots. On every
LeafWiki refresh of such a session, the OIDC refresh check refreshes with the provider,
stores a rotated provider refresh token if one is returned, and applies the returned ID
token's claims exactly like a login (allowed groups, role, verified email):

- The provider rejects the refresh (`invalid_grant`, e.g. user disabled or consent revoked)
  or the claims no longer pass `--oidc-allowed-groups` → the LeafWiki session is revoked and
  the refresh fails.
- The provider is unreachable or answers with a server error → the LeafWiki session
  continues (fail-open) and the failure is logged; the next refresh tries again. A provider
  outage therefore does not log everyone out, and the LeafWiki refresh-token lifetime remains
  the upper bound for a session that is never re-checked.

Lost access and role changes thus take effect within one access-token lifetime
(`--access-token-timeout`) instead of on the next login. Every successful OIDC login or
re-check records the time of the last provider confirmation on the identity.

**API keys.** API keys keep working as today: a key belongs to a user, and its effective
role is the intersection of the key's role and the owner's *current* role, resolved on every
request. Role changes from the provider and deactivation therefore apply to keys without
extra code. API keys never contact the provider, though, so a user removed at the provider
who no longer logs in would otherwise keep a working key. Rule, part of the admission check:
in *roles at the provider* mode, the API keys of a user with an OIDC identity work only while
the last provider confirmation is younger than `--refresh-token-timeout`, the same upper bound
a browser session has. Automation that must not depend on someone's logins uses a key of a
local account.

**Configuration.** CLI flags / ENV vars only (ADR-0011): `--oidc-issuer-url`,
`--oidc-client-id`, `--oidc-client-secret`, `--oidc-scopes`, `--oidc-username-claim`,
`--oidc-button-label`, `--oidc-auto-create`, `--oidc-default-role`, `--oidc-link-by-email`,
`--oidc-groups-claim`, `--oidc-allowed-groups`,
`--oidc-admin-groups`, `--oidc-editor-groups`, `--oidc-refresh-with-provider`,
`--oidc-token-encryption-key`, `--oidc-user-management-url`, each with a `LEAFWIKI_OIDC_*` env
var, plus the general `--disable-password-login` (`LEAFWIKI_DISABLE_PASSWORD_LOGIN`). The
feature is on exactly when `--oidc-issuer-url` is set. Startup fails if the client ID, secret
or `--public-url` (used for the redirect URI `<public-url>/api/auth/oidc/callback`) is missing,
or if the issuer is not `https://` without `--allow-insecure`; with
`--oidc-refresh-with-provider` it also fails without a valid `--oidc-token-encryption-key`.

**Placement.** Protocol client in a top-level `internal/oidc` (an optional feature without
persistent state, used by one slice, so not `internal/core`). Identity table, admission
check, credential states, session ID, refresh checks and provider-token storage in
`internal/core/auth`. Use cases and routes in a slice `internal/wiki/oidc`, which depends on
narrow interfaces it defines itself (issue a session, resolve or create an external user,
sync a role), never on `AuthService` or `UserService` directly, and which the composition root
registers only when configured. The sequence "issue CSRF cookie, set auth cookies, map HTTPS
errors" moves from the auth slice into `internal/http`, returning an error instead of writing
JSON, so the password login, the TOTP step, the refresh and the OIDC callback share it.

**Extension points introduced** (each added as its own change before OIDC uses it):

- *Login methods*: `RouterOptions.LoginMethods`, filled by the composition root and announced
  in the frontend config; the login page renders each entry through a frontend login-method
  registry (`src/lib/registries/`). The OIDC entry owns its button and its error messages.
- *User account policy*: `managedFields` and `annotations` per user (above).
- *Refresh checks*: registered with `SessionManager` (above).
- *User admission*: one check in `core/auth` at the three entry points (above).

## Alternatives considered

- **Keep SSO proxy-only (ADR-0005/0010)**: stays supported, but needs an extra component per
  instance, trusts a forgeable header, and cannot carry a per-user role without reopening the
  risk ADR-0010 closed.
- **SAML**: more complex (XML signatures, metadata exchange), and every provider operators
  name also speaks OIDC. Can be revisited as a separate login method.
- **Email or username as the identity key**: both are mutable at the provider; a changed or
  reassigned email would log someone into another person's account.
- **Server-side store for login state**: needs persistence or breaks on restart; a signed
  cookie is enough for a single short-lived round trip.
- **Admin-UI configuration**: a second configuration paradigm next to ADR-0011, plus a
  settings store that does not exist yet. Can be added later without changing the flags.
- **Several providers in v1**: no concrete need; the identity table already stores the
  issuer, so it can be added without a migration.
- **Back-channel logout instead of the refresh re-check**: the provider pushes a logout
  token to LeafWiki. Immediate, but many providers do not support it, it needs an endpoint
  the provider can reach, and it carries no role changes. Can be added later on top.
- **Fail-closed re-check** (session ends when the provider is unreachable): stricter, but a
  provider outage would log out every OIDC user within one access-token lifetime.
- **Re-check without storing the provider refresh token** (e.g. calling userinfo with the
  access token): provider access tokens expire quickly, so this only works for a short time
  after login.
- **API keys exempt from the provider** (no age rule): a user removed at the provider keeps
  a working key until someone notices.
- **No API keys for OIDC users**: safe, but breaks scripts and integrations that act as a
  person; the age rule ties keys to the same bound as sessions instead.
- **OIDC-specific checks in each login path and in the user admin**: simpler at first, but
  every further login method would add branches to the password code, the middleware and the
  user admin.
- **Role sync without overwriting local edits**: two writers for the same field, with no rule
  for which one wins.

## Consequences

- Instances without `--oidc-issuer-url` are unchanged.
- One new dependency pair (go-oidc, x/oauth2); both are pure Go and widely used.
- The general pieces (login methods, cookie helper, session ID, refresh checks, user account
  policy, admission check, deactivation, the *no local password* state, the encryption
  helper) change nothing for instances without OIDC and can ship before it.
- Accounts auto-created by reverse-proxy auth no longer accept a password reset (new accounts
  only).
- In *roles at the provider* mode, API keys of OIDC users stop working once their owner has
  not logged in or been re-checked for longer than `--refresh-token-timeout`.
- Without `--oidc-refresh-with-provider`, losing a group or being disabled at the provider
  only takes effect on the next OIDC login, at the latest when the LeafWiki refresh token
  expires (`--refresh-token-timeout`). With it, within one access-token lifetime, at the
  cost of a provider round trip per refresh, an encrypted token per session, and one more
  key to manage. While the provider is unreachable, the re-check is skipped (fail-open).
  Fully immediate revocation needs back-channel logout, which is out of scope here.
- A role change at the provider can demote the last admin; it is logged as a warning, and
  `reset-admin-password` is the way back.
- The `deactivated` state also gives password and reverse-proxy setups a way to block a user
  without deleting them.
- Operators who combine reverse-proxy auth and OIDC must know that a proxy-asserted identity
  applies per request and takes precedence.
- Revisit when operators need more than one provider, RP-initiated or back-channel logout,
  fail-closed re-checks, an
  automatic redirect to the provider, or SAML.
