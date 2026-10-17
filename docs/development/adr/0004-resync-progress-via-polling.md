# ADR-0004: Filesystem resync progress is reported via HTTP polling, not SSE/WebSocket

## Status

Accepted (retroactively documented, 2026-07-07).

## Context

Resync (rebuilding the SQLite indexes from the markdown tree — see [ADR-0001: Filesystem as Source of Truth](./0001-filesystem-source-of-truth.md)) can take a noticeable amount of time on a large wiki and runs in four phases (tree, links, tags, search). The admin UI needs to show live progress. A push-based mechanism (SSE or WebSocket) is the more common way to do this, but LeafWiki's stated principles are "minimal operational complexity" and "single binary deployment" — it deliberately avoids anything that adds operational surface for self-hosters (persistent connections, proxy buffering/timeout configuration, reconnect handling).

## Decision

The client (`stores/resync.ts`) triggers resync with `POST /api/admin/resync` (returns `202` immediately) and then polls `GET /api/admin/resync/status` every 800ms in a plain loop until the job reports `done`. The backend's `ResyncJob` (`internal/wiki/resync/job.go`) is a simple mutex-guarded status struct with no connection-oriented machinery at all.

## Consequences

- No persistent-connection handling needed on either side: no `EventSource`/WebSocket reconnect logic, no server-side hub/broadcast, nothing that needs special reverse-proxy configuration (e.g. disabling buffering for SSE) — this matters because LeafWiki is commonly deployed behind arbitrary reverse proxies by self-hosters who may not tune them.
- Progress granularity is capped at ~800ms and there's a small amount of polling traffic during a resync — both acceptable given resync is a rare, admin-triggered operation, not a hot path.
- If per-file or sub-phase progress is ever needed (finer than the current 4 coarse phases), polling every 800ms is still adequate; only a requirement for true push (e.g. progress from a resync triggered by another admin's session, reflected live in your own tab) would be a reason to revisit this and move to SSE.
