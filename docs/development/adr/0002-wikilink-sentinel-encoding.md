# ADR-0002: Represent unresolved/ambiguous wikilinks with a sentinel path, not null

## Status

Accepted (retroactively documented, 2026-07-07).

## Context

`[[Title]]`-style wikilinks resolve by title lookup against the page tree. Title lookup can fail in two distinct ways: no page currently has that title (unresolved), or more than one page shares that title (ambiguous). The link index (`internal/links`, SQLite-backed) needs to store *something* for these links — they still need to appear in link listings, be queryable, and be re-resolved later if the situation changes (a competing page gets renamed or deleted, or a new page is created).

The alternative would be to either drop unresolved/ambiguous links from the index entirely, or store `to_path` as `NULL`/empty and special-case that everywhere the column is read.

## Decision

Unresolved/ambiguous wikilinks are stored in the `LinksStore` with `to_path` set to a **sentinel string**: `"wikilink:<Title>"` (helpers in `internal/links/helpers.go`: `wikilinkSentinel`, `IsWikilinkSentinel`, `WikilinkTitleFromSentinel`). The frontend surfaces these via dedicated pseudo-protocols in the sanitizer allowlist (`wikilink-notfound:`, `wikilink-ambiguous:`).

A healing pass (`LinkService.HealWikiLinksForPage`, `HealWikiLinksForTitleIfUnambiguous`, `HealLinksForExactPath`) re-resolves sentinels back to real paths once the ambiguity/absence is resolved, invoked from the page-save orchestrator's `LinkIndexSideEffect` after renames and deletes.

## Consequences

- The link index always has a row for every wikilink written by a user, resolved or not — no special-casing "does this link exist in the index at all" throughout the codebase.
- `to_path` is not always a real filesystem path; any code reading it must check `IsWikilinkSentinel` before treating it as one. This is a sharp edge for anyone extending `internal/links` — worth checking for on any new consumer of `LinksStore`.
- Healing is coupled to the page-save flow (rename/delete), not a standalone job — an ambiguity introduced or resolved by some other means (e.g. a bulk import, direct filesystem edit outside the app) won't be healed until the next resync.

## Later changes

- **2026-10 (#1615, link state):** the sentinel is now used for *every* `[[Title]]` link, resolved
  or not, and each row carries an explicit `LinkState` (resolved / broken / ambiguous, see
  `classifyTitleMatches` in `internal/links/helpers.go`). An ambiguous link is no longer treated
  as broken; it shows up as a backlink on every matching page. The per-page healing functions
  named above were replaced by `LinkService.ReconcileTitle`, which recomputes all links to a title
  in one statement whenever the set of pages carrying that title changes. The decision itself
  (sentinel instead of `NULL`) is unchanged. Current mechanics:
  [Processes: wikilink resolution](../processes.md#wikilink-resolution--rewriting).
- **Title before path:** a target containing `/` is resolved by title first and only falls back
  to a route-path lookup when no page has that title, so titles like `TCP/IP` resolve correctly.
