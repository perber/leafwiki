# ADR-0016: State stores register once for snapshots and live restore

## Status

Proposed (2026-10-22)

## Context

LeafWiki keeps real state in SQLite stores next to `users.db`: `api_keys.db`, `favorites.db`
and `usersettings.db` ([ADR-0013](./0013-per-store-sqlite-corruption-policy.md)). Snapshots must
capture each of them and a live restore must pause, swap and reopen each of them, or a restore
silently loses or resurrects data (#1634 had to add missing settings files to backups after the
fact).

Each store was wired by hand in six places: a `snapshot.Config` path field, a vacuum block in
`createSnapshot`, a ZIP entry in `writeSnapshotZip`, a `restore.Config` field, a sanity check in
the staging step, the `swapNames` list, and four nil-guarded blocks in the restore manager
(pause before the swap, reopen after it, pause before a rollback, reopen after a rollback). With
three stores that was already three copies of the same code per place.

Share links (#1252) add a fourth store (`share_links.db`). Adding it the same way would be the
fourth copy, against the reuse rule and the Open/Closed rule in the
[conventions](../conventions.md#extensibility-strict).

## Decision

1. **`stateDBs` in `internal/restore/swap.go` is the one registration of a state database**
   (other than `users.db`): file name, the table and columns the restore sanity check probes.
   `swapNames` and `restore.StateDBFiles()` are derived from it.
2. **Snapshots take a list:** `snapshot.Config.StateDBPaths` replaces the three per-store path
   fields. The composition root fills it from `restore.StateDBFiles()`. Files that don't exist
   are skipped, as before.
3. **Live stores implement `restore.SwappableStore`** (`PauseForSwap`, `Replace(dataDir)`), which
   the existing stores already did. `restore.Config.Stores` is a list of `LiveStore{File, Store}`
   and the restore manager loops over it in the four places.
4. **The composition root lists only the stores that exist** (`Wiki.StateStores`): a disabled
   optional feature is not registered, so no nil pointer is wrapped in the interface. A test
   checks that every listed store is also in `StateDBFiles`.
5. `users.db` stays separate. Its reopen has extra steps (session invalidation, the user-label
   cache) and its own restore phase.

Adding a state store now means: one file name constant and one row in `stateDBs`, one entry in
`Wiki.StateStores` that uses the constant.

## Alternatives considered

- **Keep wiring each store by hand:** a fourth copy in six places for share links, and the next
  store would be the fifth. Rejected.
- **Let each store register its own file and schema from its package:** `restore` would have to
  import every feature package, or every feature would import `restore`. A static table in
  `restore` keeps the dependencies as they are, and the sanity check has to know the file even
  when the feature is off in the running process (its file is still swapped on disk).
- **Derive the snapshot list from the live stores:** a store whose feature is off would then be
  missing from snapshots, although its file may exist from an earlier run. Rejected.

## Consequences

- No behaviour change: the same files are captured, checked, swapped and reopened in the same
  order. Error messages now name the file (`failed to reopen favorites.db`) instead of a
  per-store wording.
- A store that is registered in `Wiki.StateStores` but missing from `stateDBs` is caught by a
  test; one in `stateDBs` without a live store is fine (feature off).
- `users.db` is still special-cased; revisit if another store needs extra steps after its
  reopen.
