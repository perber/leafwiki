# ADR-0018: Capability-URL routes run without identity, in their own route group

## Status

Proposed (2026-10-29)

## Context

Share links (#1252, [ADR-0015](./0015-scoped-read-boundary.md)) are capability URLs: the secret
in the URL (`/api/shared/<token>/…`) is the whole authorization. The person opening the link
has no LeafWiki account, and the request must be served the same way whoever sends it.

Every route today hangs off `RouterContext.Base`, which carries three middlewares registered in
`NewRouter`:

- `InjectRemoteUser` ([ADR-0005](./0005-reverse-proxy-header-auth.md),
  [ADR-0010](./0010-remote-user-auto-provisioning.md)): a trusted proxy that asserts a user
  LeafWiki doesn't know gets **401**, and with auto-provisioning on, an **account is created**.
  On a share link that would lock out a recipient who exists at the identity provider but not in
  LeafWiki, or create an account just because someone opened a link.
- `InjectAPIKeyUser`: an invalid `lw_` Bearer header answers 401 (or 429).
- `WriteGateMiddleware`: blocks writes during a live restore.

A share request must never carry an identity. Branching inside the injectors on the path
(`if strings.HasPrefix(path, "/api/shared")`) would put feature knowledge into shared
middleware, against the Open/Closed rule in the
[conventions](../conventions.md#extensibility-strict).

The secret also travels where URLs travel: the `Referer` header of outgoing links, shared
caches, search engines and the access log.

## Decision

1. **`RouterContext.CapabilityGroup(path)`** returns a route group for capability URLs at
   `BasePath + "/api" + path`. It is built from `RouterContext.Engine`, not from `Base`, so none of
   `Base`'s middlewares run: no session, no reverse-proxy identity, no API key. Engine-level
   middleware (access log, metrics, panic recovery) still applies.
2. **The group is read-only.** A write route registered in it answers 405 from the group's guard:
   it would bypass the restore write gate and CSRF, so the group refuses it by construction
   instead of by convention. Methods without a registered route (HEAD included) get the
   router's normal 404.
3. **Responses of the group's routes are hardened**, because the secret is in the URL:
   `Referrer-Policy: no-referrer`, `X-Robots-Tag: noindex, nofollow`, `Cache-Control: no-store`.
4. **The access log never shows the URL of a capability request.** The group marks the request;
   the request logger then logs the route template (`/api/shared/:token/page`) and drops the
   query. A request below a capability prefix that matches no route never runs the group's
   middleware, so the logger also knows the registered prefixes and logs only the prefix
   (`/api/shared/…`), checking the path as sent and cleaned, so `/api//shared/…` and
   `/api/shared/../…` are caught too. Prometheus
   metrics already label by route template.
5. A feature that offers capability URLs registers its routes in such a group. What the secret
   grants is the feature's decision (share links: a `scopedread.ReadScope`), not the group's.

## Alternatives considered

- **Skip lists inside `InjectRemoteUser` / `InjectAPIKeyUser`:** every injector would need the
  same list, and a new injector would have to remember it. Rejected for the group, where skipping
  is the default.
- **Let the injectors run and ignore the user in the handler:** the 401 for an unknown proxy user
  and the account auto-provisioning happen before the handler runs. Rejected.
- **A path-prefix check in the router:** a feature-specific branch in shared code. Rejected.

## Consequences

- Share links work behind an authenticating reverse proxy without a LeafWiki account, and never
  create one. The proxy in front still has to let the capability paths through unauthenticated;
  that is documented with the feature.
- Every future capability route (shared tree, shared assets) gets the same exemption and
  hardening without repeating it.
- **Known gap:** gin's panic recovery logs the request line, including the URL, at error level. A
  panic in a capability handler would therefore write the secret to the error log. Closing it
  means replacing the recovery middleware for all routes; that is left for a follow-up.
- **Known gap:** requests that match no route of the group are answered outside it, without the
  hardening headers: the router's 404, and gin's trailing-slash redirect (`…/page/` → 301 with the
  URL in `Location`). Only the link holder's own mistyped URL triggers them; changing it means
  changing redirect behaviour for the whole application.
- A feature that rate-limits failed lookups per client keys by `RemoteAddr`, like API keys. Behind
  a reverse proxy all visitors share one key: failed lookups from anyone turn into 429 for
  everyone, while valid links keep working because they are resolved first.
- A capability group has no CSRF and no write gate, so it must stay read-only (decision 2). A
  capability that needs writes needs a new decision.
