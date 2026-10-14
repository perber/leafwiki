# ADR-0001: Markdown files on disk are the source of truth; SQLite indexes are derived and rebuildable

## Status

Accepted (retroactively documented, 2026-07-07). Reflects the architecture as implemented, not a new proposal.

## Context

LeafWiki's stated principles include "Markdown stored on disk," "minimal operational complexity," and "self-hosting friendly." Pages need to support full-text search, tag lookup, arbitrary frontmatter property queries, and wikilink graph queries — none of which are efficient to serve by scanning markdown files on every request.

## Decision

The page tree (`internal/core/tree`) and revision history (`internal/core/revision`) are the only components that write the canonical representation, and they write it as plain files: markdown + frontmatter under the wiki's `root/` directory, and content-addressed blobs + JSON metadata for revisions.

Everything else that needs fast lookup — `internal/links`, `internal/tags`, `internal/properties`, `internal/search` — keeps its own per-wiki SQLite database. These databases are treated as **caches derived from the filesystem**, not as additional sources of truth. A dedicated resync mechanism (`internal/wiki/resync`, `Wiki.reloadWithProgress`) can always fully rebuild them by walking the tree from scratch.

## Consequences

- Backup is simple: `internal/backup`'s git-based backup only needs to version the markdown tree; the SQLite indexes don't need to be backed up (they're reproducible).
- Any SQLite index can become inconsistent (crash mid-write, manual file edits outside the app, a bug in a side effect) without being a data-loss event — it's fixed by a resync, not a restore.
- This is *why* resync exists as a first-class, admin-triggerable, signal-triggerable operation (see [ADR-0004: Resync Progress via Polling](./0004-resync-progress-via-polling.md)) rather than an internal implementation detail.
- The page-save orchestrator (see [processes: page save](../processes.md#page-save)) treats index-update side effects as best-effort (log-and-continue on failure) specifically because they're recoverable via resync — this would be the wrong tradeoff if the SQLite state were authoritative.
- Cost: two storage engines to reason about instead of one; a change to "how a page's derived data is computed" (e.g. new tag extraction rule) requires either a resync or careful incremental-update code in the corresponding side effect to avoid drift until the next resync.
