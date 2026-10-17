# ADR-0008: Structured logging schema — levels, field naming, and when to log

## Status

Accepted (2026-07-07). Retroactively documents and standardizes existing backend `log/slog` usage; establishes the convention to follow going forward. The frontend gap noted below is flagged, not solved, by this ADR.

## Context

The backend already uses `log/slog` extensively (`internal/importer`, `internal/core/tree`, `internal/core/revision`, `internal/core/assets`, `internal/links`, `internal/search`, `internal/http/...`, `internal/backup`, ...). `cmd/leafwiki/main.go`'s `setupLogger()` wires a JSON handler to stdout with level controlled by `LEAFWIKI_LOG_LEVEL` (`debug`/`warn`/`error`, default `info`). Several services already hold a component-scoped logger (`slog.Default().With("component", "AssetService")`).

But there was never a documented convention, so usage has drifted:

- Field key casing is inconsistent: most packages use camelCase (`pageID`, `nodeID`, `oldSlug`, `fromVersion`), but `internal/importer` uses snake_case (`source_path`, `target_path`, `workspace`).
- There's no rule for *when* to log an error versus just returning it. Some errors are logged at a swallow point (e.g. a `defer file.Close()` failure that has nowhere else to go) — that's correct — but nothing stops the same failure from being logged again further up the call chain if a future change adds a log call there too.
- The frontend has no logging abstraction at all: roughly 30 raw `console.log`/`console.warn`/`console.error`/`console.debug` calls scattered across stores and features, inconsistent prefixing, no verbosity gating.

LeafWiki is self-hosted (see `AGENTS.md`); the primary reader of these logs is an operator tailing `docker logs` or a systemd journal, not a developer attached with a debugger. Logs need to be useful in that context specifically.

## Decision

### Backend (`log/slog`)

1. **Level semantics:**
   - `Debug` — internal detail only useful when actively diagnosing a specific issue; off by default. Example: path-resolution steps, revision-coalescing decisions.
   - `Info` — a notable lifecycle or state-change event an operator benefits from seeing by default (schema migration ran, backup initialized, import completed). Not routine per-request traffic — that already has its own dedicated line in `internal/http/router.go`.
   - `Warn` — something recoverable happened: the operation continued via a fallback, retry, or skip, but an operator should know (corrupt index rebuilt, untrusted proxy request skipped, unresolvable import resource skipped).
   - `Error` — an operation failed. Log an error **only at the point it is swallowed**, i.e. not also returned or propagated further (deferred cleanup, a background job, a goroutine with no caller to report to). If a function returns the error to its caller, don't also log it there — log it once, at whichever point stops propagating it, so one root-cause failure doesn't produce multiple log lines.
2. **Field naming:** camelCase keys (`pageID`, `nodeID`, `oldSlug`, `newSlug`, `fromVersion`), matching the majority of existing call sites. `internal/importer`'s snake_case fields are the known outlier — migrate opportunistically when next touching that file rather than as a standalone rename.
3. **Component scoping:** any package/service that logs more than an isolated one-off call should hold its own `*slog.Logger`, set once via `slog.Default().With("component", "<Name>")` in its constructor (following the existing `AssetService`/`TreeService`/importer `Planner`/`Executor` pattern) instead of calling `slog.Default()` inline at each call site.
4. **Error field key:** always attach the Go `error` under the literal key `"error"` — already universal in the codebase, keep it.
5. **Don't log speculatively.** If a log line wouldn't be actionable to an operator reading it, it doesn't belong — use a test to pin down internal correctness, not a log line.

### Frontend

No logging abstraction exists yet; introducing one (e.g. a thin `lib/logger.ts` wrapper with the same Debug/Info/Warn/Error semantics as above) is out of scope for this ADR and should be its own follow-up before `console.*` usage grows further. Until then, apply the same swallow-point rule: `console.error`/`console.warn` at the point an error is caught and not rethrown or surfaced via UI state, not redundantly at multiple layers.

## Consequences

- New backend code gets a scoped logger and camelCase fields by default; reviewers can flag ad-hoc `slog.Default()` calls and importer-style snake_case drift as inconsistent with this ADR.
- The "log once, at the swallow point" rule prevents duplicate log lines for a single failure and forces a decision, at implementation time, about where an error's handling actually terminates — this is what `AGENTS.md`'s "decide on logging while implementing" rule is checking for.
- Frontend logging stays unstandardized for now; this is a known, tracked gap, not a silent omission.
- `LEAFWIKI_LOG_LEVEL` remains the only verbosity lever; this decision does not introduce per-component level overrides.
