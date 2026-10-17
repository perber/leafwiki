# ADR-0003: Revision coalescing uses a fixed per-author time window, not per-edit debounce

## Status

Accepted (retroactively documented, 2026-07-07). Superseded an earlier debounce-based attempt during development — see History.

## Context

The editor auto-saves on a 2-second debounce ([processes: editor auto-save](../processes.md#editor-auto-save)). Without grouping, a single editing session of several minutes would create dozens of near-duplicate content revisions, defeating the purpose of a revision history (finding a meaningful prior version) and bloating on-disk storage.

## Decision

`internal/core/revision.Service` coalesces a new content-update revision into the immediately preceding one when all of the following hold: coalescing is enabled (`coalesceWindow > 0`), the previous revision is itself a `RevisionTypeContentUpdate`, the author is the same, and the elapsed time since the previous revision's `CreatedAt` is within `coalesceWindow` (default `5m`, configurable via `--revision-coalesce-window` / `LEAFWIKI_REVISION_COALESCE_WINDOW`, 0 disables it entirely).

Coalescing reuses the previous revision's ID and overwrites its content blob rather than creating a new revision record, with old-blob GC and a per-page mutex to keep this safe under concurrent saves.

## History

During development (`feat/revision-coalescing` branch) a debounce-based coalesce window (reset on every save, rather than measured from the *first* revision in the group) was tried and reverted back to the fixed-interval approach documented here, after concurrency/correctness issues.

## Consequences

- Revision counts stay predictable and proportional to actual editing sessions, not keystroke/debounce frequency.
- A single continuous editing session that spans longer than the coalesce window will still produce multiple revisions — accepted as the simpler, more predictable tradeoff over a sliding/debounced window.
- Coalescing logic is a source of real complexity (per-page locking, blob GC, no-op short-circuiting) concentrated in one well-tested area (`internal/core/revision/service_test.go`) — future changes to save frequency (e.g. a shorter auto-save debounce) should be checked against the default coalesce window to make sure the ratio still makes sense.
