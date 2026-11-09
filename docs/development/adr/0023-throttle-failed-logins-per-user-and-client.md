# ADR-0023: Throttle failed logins per account and client, and resolve the client through the trusted proxies

## Status

Proposed (2026-11-09)

## Context

Native login counted failures per account: five wrong passwords locked the account for 15
minutes, for every caller. That keeps guessing slow, but it also means that anyone who knows an
account name can keep its owner out by sending a few wrong passwords every 15 minutes. The lock
protects the account by making it unavailable to everyone, the real owner included.

A per-client wait avoids that: a client that fails repeatedly waits longer, and nobody else is
affected. It only works if "client" means something. `c.ClientIP()` in gin trusts every peer by
default, so a direct client could pick its own address by writing it into `X-Forwarded-For` or
`X-Real-IP`; a throttle keyed on that address would be weaker than the lock it replaces. The
address therefore has to be resolved through a list of proxies that are allowed to say who the
client is.

LeafWiki already has such a list: `--trusted-proxy-ips`, introduced for remote-user header auth
([ADR-0005](./0005-reverse-proxy-header-auth.md)). ADR-0005 names the sharp edge of that list:
security depends on it being correct. The same edge applies here in both directions. A list that
is too broad (for example `0.0.0.0/0`) lets a client choose its own address again. A list that
leaves out a hop of the chain, typically the reverse proxy itself, resolves every visitor to that
hop, so they all share one client.

`--trusted-proxy-ips` is empty by default, and until now it was only needed for remote-user auth.
The common Docker, Caddy or nginx installation therefore runs with the reverse proxy unlisted.

The second login step (TOTP code) is different from the first: reaching it takes the account's
password. A 6-digit code accepts a few values per attempt, so the number of guesses matters more
there, and locking the account at that step does not let a stranger keep the owner out.

Code locations: `internal/core/auth/login_attempt_tracker.go`, `internal/core/auth/auth_service.go`
(`Login`, `CompleteTOTPLogin`), `internal/http/client_address.go`,
`internal/http/middleware/security/client_bucket.go`.

## Decision

**Password step: a decaying backoff per (account, client).**

- The key is the account ID (so username and email alias to one entry) and a client value
  computed in the HTTP layer by `security.ClientBucket`: the address from `c.ClientIP()`, with an
  IPv4-mapped IPv6 address unmapped, an IPv6 address masked to its /64, and anything empty,
  unparsable or zoned mapped to one shared `unknown` client. `internal/core/auth` only sees the
  opaque, already grouped `auth.Client` and never parses an address.
- The first 5 attempts carry no wait. After that the wait is 5 s and doubles with every further
  counted attempt, up to 15 min. An entry is forgotten after one hour without a counted attempt.
  The constants live in one block in `login_attempt_tracker.go`;
  `loginBackoffDecay >= loginBackoffCap` is asserted by a test, so tuning cannot make an entry
  forgotten while it is still waiting.
- The attempt is counted before the password is verified, in one critical section, so parallel
  guesses cannot pass the check before the first one is recorded. During a wait even the correct
  password is refused (otherwise the wait would not slow guessing down), and a refused call is
  neither counted nor does it extend the wait.
- A correct password clears that (account, client) entry, whether or not TOTP is enabled.
- A refusal is answered exactly like invalid credentials: same status, body and headers, no
  `Retry-After`, and the same bcrypt work, so neither the answer nor its timing says that the
  account exists or that a wait is running. The metric label `outcome="locked"` is kept and now
  counts throttled attempts.

**TOTP step: a short lock per account, separate from the password counter.** Five counted code
attempts refuse the account for 15 minutes, for every client, counted before verification and
cleared only by a full TOTP success. Wrong passwords never use up code attempts, and wrong codes
never use up password attempts; a correct password does not touch this lock.

**Storage: in memory, bounded.** No SQLite: a restart clears the throttle, as the old lock was
cleared, and nobody can cause a restart by failing to log in. The password table holds at most
10 000 entries, because the client part of the key comes from the network. On insert at the
limit, expired entries are dropped first; if none are, the entry that will be allowed again
soonest goes, ties broken by the earliest last attempt and then by key, so the choice does not
depend on map order. The table never refuses to track and never fails open. The sweep happens on
insert; there is no background goroutine, so with a full table each insert scans it once under the
lock, which is far cheaper than the bcrypt comparison that follows. The clock is an injectable
field on the tracker.

**Client address: gin's resolution, configured from `--trusted-proxy-ips`.** `configureClientAddress`
passes the parsed list to `engine.SetTrustedProxies` right after the engine is created. A peer
that is not listed resolves to its TCP peer address, and an empty list trusts nobody.
`TrustedProxies.Entries` rebuilds the entries from the parsed values, because gin appends `/32`
or `/128` to the original text and an IPv4-mapped IPv6 entry would otherwise become a wide IPv6
prefix. Only `X-Forwarded-For` is read, so a client-sent `X-Real-IP` is never used. Gin walks
`X-Forwarded-For` from the right and stops at the first address that is not listed, so every hop
of the chain (CDN and proxy) must be listed, and the proxy must append to or overwrite the
header (nginx: `$proxy_add_x_forwarded_for`). The first request that carries a forwarding header
from an unlisted peer logs one warning that the header was ignored and that a reverse proxy has
to be listed. Gin trusts every unix-socket peer, and such a peer has no IP the list could name, so
on a unix socket the header is read only when at least one proxy is listed (otherwise every
socket client shares the `unknown` bucket instead of choosing its own address). The flag now also decides where `X-Forwarded-For` is believed; ADR-0005 gets a
dated entry that points here.

## Alternatives considered

- **Keep the per-account hard lock**: a third party who knows the account name keeps the owner
  out, and the owner has no way to get in short of waiting or an admin reset.
- **Keep the hard lock behind an opt-in flag**: doubles the code, tests and docs for a mode that
  nobody is recommended to use. An operator who wants it can still run a proxy-level limiter.
- **A per-account aggregate cap on the password step** (a limit on failures across all clients):
  any cap that refuses all clients is the account-wide lock again.
- **A per-IP limiter only**: the existing limiter already does this per host. It says nothing
  about how many guesses one account receives, and its count is cleared by any successful
  response, including the `200` that opens the TOTP step.
- **CAPTCHA**: a new dependency and a UI step for a self-hosted, single-binary wiki, and it still
  needs a per-client notion of who is asking.
- **SQLite-backed state**: survives restarts, but adds a store with a corruption policy
  ([ADR-0013](./0013-per-store-sqlite-corruption-policy.md)), a write on every failed login, and
  coordination with live restore, for state that is meant to be short-lived.
- **TOTP on the per-client backoff**: removes the third-party wait at that step too, but a
  guesser who already holds the password and many addresses would get about a million code
  guesses a day, where the per-account lock allows 480 in total. The second factor would be
  weakened to remove a problem that does not exist at that step.

## Consequences

- A locked-out owner is no longer locked out by someone else: a client's wrong passwords only
  slow down that client.
- The protection is fixed once the reverse proxy is listed in `--trusted-proxy-ips`.
  `--trusted-proxy-ips` is empty by default, so an installation behind a reverse proxy that is
  not listed resolves every visitor to the proxy address and they share one client. A third party
  then needs one attempt right after each wait expires to keep the owner at the longest wait (a
  refused call does not extend it): up to 15 minutes at a time, not an account lock, but still
  a wait that is not the owner's doing. The once-only warning, the flag text and
  [processes.md](../processes.md) point at the fix, which is configuration.
- A listed range that is too broad (for example `0.0.0.0/0`) lets a client choose its own address
  again; a chain with a hop missing collapses to that hop. This is the ADR-0005 sharp edge.
- Guessing a password from many addresses is weaker than before. Per client bucket, a steady
  guesser gets about 4 attempts per hour (about 107 per day), and one that stops before the longer
  waits and lets the entry decay gets about 10 per hour (about 245 per day), against 20 per hour
  (480 per day) with the per-account lock. A /56 or /48 handed to one subscriber yields 256 to
  65 536 client values. What remains: the per-host login limiter, the cost of bcrypt, the TOTP
  step (not weakened), and the `outcome` metric. A log or metric alert for one account failing
  across many clients is a possible follow-up.
- Someone who knows the password can keep the owner out of the TOTP step with wrong codes until
  the password is reset. That account's password is already known to them, so this adds a wait,
  not a new way in.
- A restart clears the throttle. A restored `users.db` can carry other account IDs; stale entries
  are harmless and age out.
- Other places that check a password (changing it, confirming or disabling TOTP) do not use this
  tracker; they sit behind a session and the per-host limiter.
- The metric label `locked` no longer describes what happens; it is kept for existing
  dashboards.
- Revisit when the numbers need tuning, when a second consumer of the client address appears (for
  example the rate limiters keying on `ClientBucket`), or if the default of an empty
  `--trusted-proxy-ips` changes.
