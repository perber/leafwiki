# Architecture Decision Records

An ADR records one architectural decision: the problem, what was chosen, what was rejected and
what it costs. Before changing something that looks like an odd choice, check here. It may be a
deliberate tradeoff, and changing it means writing a new ADR that supersedes the old one.

## When an ADR is required

Write an ADR when a change does at least one of these:

- **Changes the on-disk format or storage model**: Markdown/frontmatter layout, link format,
  folder structure, what is source of truth vs. derived index.
- **Changes a contract with existing installations**: a migration, a breaking change to CLI
  flags, env vars, HTTP API or config, anything users have to act on when upgrading.
- **Touches the security or auth model**: authentication paths, sessions, roles, trust
  boundaries (e.g. reverse-proxy headers), what is publicly reachable.
- **Adds a dependency with lasting weight**: a new runtime service, a database, a large frontend
  library, a new protocol (SSE, WebSockets, …), or anything that adds operational work for
  self-hosters.
- **Sets a project-wide pattern** that other code is expected to follow: error handling,
  logging, state management, extension points, testing approach.
- **Deliberately chooses the less obvious option** in a way someone will want to "fix" later
  (polling instead of SSE, sentinel instead of null, …).

No ADR is needed for bug fixes, refactors that keep behavior, new features that follow existing
patterns, or dependency bumps. When in doubt, ask on the issue; an ADR that turns out to be
unnecessary costs little.

## Process

1. Copy [template.md](./template.md) to `NNNN-short-title.md` (next free number, kebab-case).
2. Status `Proposed`. Open it in the same PR as the change, or in a PR of its own before the
   implementation if the decision should be agreed first.
3. On merge the status becomes `Accepted (YYYY-MM-DD)`.
4. ADRs are not rewritten after acceptance. To change a decision, write a new ADR and set the old
   one to `Superseded by [ADR-NNNN](./NNNN-….md)`. Small factual corrections (renamed file,
   fixed link) are fine. When the implementation moves on but the decision still holds, add a
   dated entry under a `## Later changes` section at the end instead of editing the original
   text.
5. Add the ADR to the index below.

Status values: `Proposed`, `Accepted`, `Rejected`, `Deprecated`, `Superseded by ADR-NNNN`.

## Index

| ADR | Decision | Status |
|---|---|---|
| [0001](./0001-filesystem-source-of-truth.md) | Markdown files on disk are the source of truth; SQLite indexes are derived and rebuildable | Accepted |
| [0002](./0002-wikilink-sentinel-encoding.md) | Unresolved/ambiguous wikilinks use a sentinel path, not null | Accepted |
| [0003](./0003-revision-coalescing-fixed-window.md) | Revision coalescing uses a fixed per-author time window | Accepted |
| [0004](./0004-resync-progress-via-polling.md) | Resync progress via HTTP polling, not SSE/WebSocket | Accepted |
| [0005](./0005-reverse-proxy-header-auth.md) | Reverse-proxy header auth as an alternate front door, not a second auth system | Accepted |
| [0006](./0006-zustand-feature-local-stores.md) | Frontend state is per-feature Zustand stores that call each other directly | Accepted |
| [0007](./0007-extension-points-long-term-goal.md) | Long-term goal: explicit extension points for backend and frontend | Accepted (direction) |
| [0008](./0008-structured-logging-schema.md) | Structured logging schema: levels, field naming, when to log | Accepted |
| [0009](./0009-session-manager-extraction.md) | SessionManager extracted from AuthService as seam for other identity providers | Accepted |
| [0010](./0010-remote-user-auto-provisioning.md) | Opt-in auto-provisioning for reverse-proxy header auth | Accepted |
| [0011](./0011-smtp-cli-env-optional-feature.md) | SMTP as a CLI/ENV-only optional feature | Accepted |
| [0012](./0012-portable-markdown-link-format.md) | Portable on-disk Markdown link format | Proposed |
| [0013](./0013-per-store-sqlite-corruption-policy.md) | Each SQLite store chooses its corruption policy; real state is never deleted automatically | Accepted |
| [0014](./0014-generated-api-documentation.md) | API documentation is generated (Swagger 2.0, pinned swag) and covered incrementally with exact exceptions | Proposed |
