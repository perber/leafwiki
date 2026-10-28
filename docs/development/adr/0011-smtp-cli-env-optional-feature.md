# ADR-0011: SMTP as a CLI/ENV-Only Optional Feature

## Status

Accepted (2026-08-18).

## Context

Self-service password-reset and user-invite (GitHub Discussion #1331) both need an outgoing
SMTP integration. The maintainer's public reply to that discussion set one hard constraint: SMTP
must be a pure *addition* — LeafWiki keeps working with zero email configuration for
self-hosters who don't want it, matching the project's "minimal operational complexity"
principle (see `CONTRIBUTING.md`).

LeafWiki already has a precedent for this exact shape of decision: `--enable-api-key-management`
and `--enable-http-remote-user` are both CLI-flag/ENV-var-gated optional features with no
database-stored, UI-configurable settings — see [ADR-0005: Reverse-Proxy Header Auth](./0005-reverse-proxy-header-auth.md) for
the reverse-proxy case specifically. Adding a third, UI-configurable path (e.g. an admin
Settings page for SMTP credentials, stored in SQLite) would be a second, inconsistent way to
configure optional server-level features.

## Decision

SMTP config (`internal/core/email.Config`) is populated exclusively from CLI flags / ENV vars
in `cmd/leafwiki/main.go` (`--smtp-host`, `--smtp-port`, `--smtp-username`, `--smtp-password`,
`--smtp-from`, `--smtp-from-name`, `--smtp-security`, `--smtp-insecure-skip-verify`,
`--smtp-timeout`, `--public-url`), following the same `resolveString`/`resolveBool`/
`resolveDuration` CLI-then-ENV-then-default helpers every other optional feature already
uses. `Config.Enabled()` is simply `Host != ""` — no separate on/off flag. A new
`validateSMTPConfig` fails fast at startup (mirroring `validateHTTPRemoteUserConfig`) if
`--smtp-host` is set but `--smtp-from`/`--public-url` are missing, or `--public-url` isn't an
absolute `http(s)://` URL — a bad public URL would otherwise only surface once already
embedded in a sent email, which can't be un-sent.

There is no database table, no admin-UI form, and no runtime way to change SMTP settings
short of restarting the process with different flags/env — same tradeoff the existing
git-backup and reverse-proxy-auth features already made.

## Consequences

- Self-hosters who don't set `--smtp-host` get the exact same LeafWiki they had before this
  feature existed — `smtpEnabled` is `false`, the frontend hides every reset/invite-related
  UI element, and the four new HTTP routes return a clean `403 auth_email_disabled` instead
  of ever touching a nonexistent mailer.
- Credentials (SMTP password) stay ENV/flag-only, with the same operational risk profile as
  the pre-existing `--git-backup-ssh-key` (process-listing visibility if passed as a flag
  rather than env var — `main.go` already warns about this for the git-backup case; the same
  caveat applies here and should get the same warning if this becomes a support question).
- Changing SMTP settings requires a restart, not a form submission — acceptable for a
  self-hosted single-tenant deployment; would need revisiting if per-tenant SMTP ever became
  a Managed Hosting requirement (currently no signal that it is).
- Consistent with the existing "optional feature = CLI/ENV flag" pattern means an operator
  who already knows how `--enable-api-key-management` or `--enable-http-remote-user` work
  doesn't need to learn a new configuration paradigm for this one.

## Later changes

- **Flag definitions moved:** the CLI moved to `urfave/cli/v3`. The SMTP flags now live in
  `cmd/leafwiki/flags_email.go` and read their env var via `Sources: envVars(...)` instead of the
  `resolveString`/`resolveBool`/`resolveDuration` helpers. `validateSMTPConfig` is still in
  `cmd/leafwiki/main.go`. The decision (CLI/ENV only, no UI) is unchanged.
- **2026-10 (#1292, `--public-url` moved):** `--public-url` is now a server flag
  (`cmd/leafwiki/flags_server.go`), because it describes the wiki itself, not the mail
  transport. Password-reset and invite emails still require it when `--smtp-host` is set and it
  is validated the same way. Search-engine rendering uses it as the only source for canonical
  links and sitemap URLs ([ADR-0018](./0018-search-engine-rendering-for-public-wikis.md)).
- **2026-10 (#1292, `--public-url` checked without SMTP):** `validatePublicURL` checks the
  value whether or not SMTP is configured. With `--smtp-host` an invalid value is still a
  startup error with the same message; without it the value is ignored with a warning. A path
  that differs from `--base-path` logs a warning. A password in the URL is redacted in logs and
  errors.
