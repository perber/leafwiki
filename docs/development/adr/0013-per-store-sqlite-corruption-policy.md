# ADR-0013: Each SQLite store chooses its corruption policy; real state is never deleted automatically

## Status

Accepted (2026-10-17)

## Context

LeafWiki keeps two kinds of SQLite databases in the data directory (see
[architecture](../architecture.md#storage-model) and
[ADR-0001](./0001-filesystem-source-of-truth.md)):

- **Derived indexes** (`links`, `tags`, `properties`, `search`): rebuilt from the Markdown files
  by a resync. Losing one costs a rebuild, no data.
- **Real state** (`users.db`, `sessions.db`, `email_tokens.db`, `favorites.db`,
  `usersettings.db`, `api_keys.db`): nothing else holds this information. Losing it is data loss
  that only a backup can undo.

All store constructors except `NewAPIKeyStore` shared one recovery helper in
`internal/core/shared/sqliteutil`. On a SQLite I/O or corruption error (`SQLITE_IOERR`,
`SQLITE_CORRUPT`, `SQLITE_NOTADB`) it deleted the database plus its `-journal`/`-wal`/`-shm`
sidecars and opened a fresh, empty one. The constructor then reported success.

That is right for derived indexes and wrong for real state:

- An unreadable `users.db` was replaced by an empty one, and `InitDefaultAdmin` then created a
  fresh admin from the startup flags. That is a data loss and an auth-model change in one step.
- `SQLITE_IOERR` does not prove the file is broken (read-only mount, failing SD card on a
  Raspberry Pi, a full disk). Deleting on it destroys evidence an operator could still recover.
- A `-wal` sidecar can hold committed transactions; deleting it loses them even when the main
  file is fine.
- Deleting the original also removes the operator's way out: restoring a snapshot or moving the
  file aside and inspecting it.

The policy lived in shared code with no knowledge of which store was calling, so every new store
inherited delete-on-corruption by default.

## Decision

### The seam

`internal/core/shared/sqliteutil` offers:

```go
type CorruptionPolicy func(dbPath string, err error) (retry bool, out error)

func OpenWithPolicy(dbPath string, policy CorruptionPolicy, open func() error) error
```

Every store passes its policy explicitly at its constructor. There is no default and no shared
helper that picks a policy by name or kind, so adding a store means choosing, not inheriting.
The old `RetryOnCorruption` helper is removed.

Contract (doc comment on `CorruptionPolicy`, enforced by a table test):

- `OpenWithPolicy` classifies the error itself (`IsSQLiteRecoverableError`) and calls the policy
  only for recoverable errors.
- Before `open` runs, `OpenWithPolicy` opens an existing, non-empty file read-only and reads its
  schema; a recoverable error there goes to the policy before `open` runs at all. A read-write
  connection to a database whose header SQLite cannot read discards the `-wal`, even one that
  holds committed transactions, so without the probe the sidecar would be gone before any policy
  could keep it. A broken header that the WAL can still repair passes the probe and is left to
  SQLite. Missing and empty files are valid new databases and are not probed, except an empty
  file next to a non-empty `-wal` or `-journal`: such a sidecar can only belong to a database
  that held data, so the probe reports it as a recoverable error and the policy decides. The probe resolves
  the path to an absolute `file:` URI (the default `--data-dir` is the relative `./data`) and
  strips a Windows `\\?\` prefix. It fails closed: if it cannot read an existing, non-empty file
  for a non-recoverable reason, `OpenWithPolicy` returns that error and `open` never runs. Only
  `SQLITE_READONLY_*` (a hot journal or a WAL that needs recovery) lets `open` run, because only
  a read-write connection can do that recovery. Failing closed applies to every store, derived
  ones included: a transient error such as `SQLITE_BUSY` during the probe stops startup instead
  of rebuilding the index. The probe only catches corruption in the header
  or the schema; corruption deeper in the file is found by `open`, after a read-write connection
  was made.
- A nil policy panics at the start of `OpenWithPolicy`, so any constructor test catches a store
  that forgot to choose.
- It retries at most once.
- `retry=true` means the policy has removed or moved the database and all sidecars. A policy
  that could not do so returns `retry=false` with an error instead (`RebuildOnCorruption` when a
  file cannot be deleted, `QuarantineOnCorruption` when a rename fails). No handle is open when
  the policy runs (the probe closes its own; `open` closes its handle before returning an
  error). `out` is ignored.
- `retry=false` returns `out`. `out == nil` is a contract violation; `OpenWithPolicy` then returns
  the original error, so a faulty policy can never make a broken store look healthy.
- Non-recoverable errors and the error of the retry pass through unchanged.

### Policies per store

| Store | Policy | On a recoverable error |
|---|---|---|
| `links`, `tags`, `properties`, `search` | `RebuildOnCorruption` | delete db + sidecars, retry, log one `Warn` with `path`, `sqliteCode`, `error` |
| `users.db` | `FailOnCorruption` | touch nothing (the probe may leave a `-shm`, see Consequences); return `*CorruptDatabaseError{Path, Err}`; startup fails |
| `sessions.db`, `email_tokens.db`, `favorites.db`, `usersettings.db` | `QuarantineOnCorruption` | rename db and every existing sidecar to `<name>.corrupt-<UTC timestamp>` (sidecars as `<name>.corrupt-<UTC timestamp>-wal` etc.), retry with an empty db, log one `Error` with `path`, `quarantinedPath`, `sqliteCode`, `error`; if a rename fails, move the renamed files back and return `*CorruptDatabaseError`; files that cannot be moved back are listed in its `Stranded` field and in the operator message |
| `api_keys.db` | none: `NewAPIKeyStore` does not use the seam | any open error, corruption included, fails startup and touches nothing |

- **`users.db` fails**, it is never moved aside automatically: an empty user store would let
  `InitDefaultAdmin` create a new admin from the startup flags. The operator decides
  (`restore-snapshot`, or move the file aside by hand).
- **Quarantine** keeps the bytes for later inspection or manual recovery while the instance keeps
  running. Sessions and email tokens are short-lived (a restore already invalidates sessions);
  favorites and user settings are worth keeping but not worth refusing to start over. The
  timestamp has no `:` (e.g. `20261017T143005Z`) so the name is valid on Windows. The handle is
  closed before the rename, because Windows cannot rename an open file.
- **Logging (ADR-0008):** `FailOnCorruption` does not log; the error propagates and is reported
  once at the edge. `RebuildOnCorruption` logs `Warn` because the store recovers fully.
  `QuarantineOnCorruption` logs `Error` because user data is no longer live, even though startup
  continues. Policies log through `slog.Default()`: the `CorruptionPolicy` signature has no
  logger, and the stores that hold an injected logger (favorites, user settings) use one derived
  from the default anyway, so the line ends up in the same place.

### Operator message at the edge

`sqliteutil` returns the typed error. `NewWiki` and `tools.ResetAdminPassword` wrap it with `%w`.
`cmd/leafwiki` finds it with `errors.As` and prints the path, the SQLite code and the ways out.
It is not a `LocalizedError`: it is a startup error with no HTTP response and no i18n path.

### Rollout

1. The seam and `RebuildOnCorruption`, with every existing store passing it explicitly (no
   behaviour change).
2. One PR: `RebuildOnCorruption` stops retrying when a file cannot be deleted,
   `QuarantineOnCorruption`, the read-only probe, `FailOnCorruption` for `users.db`, the
   quarantine policy for sessions, email tokens, favorites and user settings, the CLI message and
   docs.

## Alternatives considered

- **A `derived bool` flag (or a store-kind enum) on the shared helper**: a `switch` in shared code
  that must learn every new kind, and `false` would become a silent default again.
- **Fail for every real-state store**: safest, but a broken `sessions.db` would keep the whole
  wiki offline over data that is thrown away on every restore anyway.
- **Quarantine `users.db` too**: still starts with an empty user store and so still creates a
  fresh admin from the flags. Rejected for the auth reason above.
- **Try to repair the file** (`.recover`, `VACUUM INTO`): not available in the pure-Go driver
  without extra tooling, can silently drop rows, and hides the incident from the operator.
- **Treat `SQLITE_IOERR` as non-recoverable for all stores**: arguably right, but it changes
  derived-index behaviour too and is a separate decision.

## Consequences

- No store can lose real state through an automatic delete; a new store has to pick a policy at
  its constructor, and reviewers can see the choice.
- Existing installations whose `users.db` is unreadable stop at startup instead of coming up with
  a fresh admin. This needs a release note.
- A transient `SQLITE_IOERR` (read-only mount, failing storage) on `users.db` now stops startup.
  Accepted: refusing to start is recoverable, wiping users is not.
- Quarantined files accumulate in the data directory until an operator removes them; they are
  small and named so they are easy to find.
- The read-only probe costs one extra open per store at startup and may leave a `-shm` next to a
  database it rejected; `-shm` is an index SQLite rebuilds from the `-wal`, not data.
- Two quarantines of the same store within the same second would collide; the second one then
  fails instead of overwriting the first.
- Derived indexes still delete on `SQLITE_IOERR`. Revisit if that causes avoidable rebuilds on
  flaky storage.
- Lazy reopen paths (`UserStore.Connect`, the stores' `withDB` helpers) do not go through the
  policy; an error there surfaces as a failed request and touches no files. Stores rebuilt after
  a restore go through their constructor and so through their policy.
