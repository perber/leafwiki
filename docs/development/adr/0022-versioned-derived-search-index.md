# ADR-0022: The search index persists across restarts and is rebuilt on a schema version or DDL mismatch or on corruption

## Status

Proposed (2026-11-21)

## Context

`search.db` is a derived index ([ADR-0001](./0001-filesystem-source-of-truth.md)): an FTS5
table `pages` in `internal/search/sqlite_index.go`, filled from the Markdown files.

- Until now `ensureSchema` ran `DROP TABLE IF EXISTS pages` and recreated it on every open, so
  every process start threw the index away. Boot then refilled it in a background goroutine
  (`initSearch` in `internal/wiki/wiki_boot.go`, `SearchIndexSideEffect.IndexAllPagesContext`).
  Until that finished, search returned partial results. The drop was also the only thing that
  kept the table in the shape the binary expected: there was no record of which schema a file
  had.
- The planned search work makes indexing slower and the stored text binary-specific: a Go
  normaliser decides which tokens are written into the index, a contentless table holds only
  normalised tokens, and a trigram table is added. Reindexing everything on every boot gets more
  expensive, and an index written by a different normaliser gives wrong results without any
  error.
- The bundled `modernc.org/sqlite` driver has no FTS5 tokenizer API, so the normalisation runs
  in Go and its output lands in the database. SQLite cannot tell that it changed; only the
  binary can.
- The boot path will later trust the persisted rows and only sync what changed (a fingerprint
  sync, see Later changes). That needs a reliable answer to "was this file written in the shape
  and normalisation I expect?".

## Decision

- `internal/search` declares `const searchSchemaVersion` and stores it in search.db's
  `PRAGMA user_version`. `searchSchema` is the one place that describes every search table.
- On open, `ensureSchema` takes the write lock with `BEGIN IMMEDIATE` on one connection and,
  inside that transaction, keeps the tables only if **both** hold:
  - `user_version` equals `searchSchemaVersion`;
  - our schema objects in `sqlite_master` (type, name, table and SQL of our tables, views,
    indexes and triggers, the FTS5 shadow tables `pages_*` included) equal what SQLite stores
    when `searchSchema` is created in an empty in-memory database. Comparing SQLite's own text
    avoids depending on how SQLite normalises `CREATE` statements. SQLite's internal `sqlite_*`
    objects are not part of the comparison: `sqlite_stat1`/`sqlite_stat4` appear after
    `ANALYZE` or `PRAGMA optimize`, and `sqlite_sequence` and `sqlite_autoindex_*` follow from
    the DDL that is compared anyway.
- Otherwise, in the same transaction, it drops every table and view of our schema (virtual
  tables first, which drops their shadow tables; `sqlite_*` objects are left to SQLite), creates
  `searchSchema` and sets `user_version`, and logs one `Info` line when there was something to
  drop (ADR-0008). Because the check runs under the write lock, a second process or instance
  opening the same outdated file waits (`busy_timeout`) instead of failing with `SQLITE_BUSY`,
  then sees the schema the first one rebuilt and keeps it; only one of them rebuilds.
- The DDL check is needed on its own: a binary from before this ADR drops and recreates `pages`
  in its own shape and leaves `user_version` untouched.
- **What triggers a rebuild:** a different `user_version`, a different schema, or a corrupt file.
  Corruption is unchanged: `NewSQLiteIndex` passes `RebuildOnCorruption`
  ([ADR-0013](./0013-per-store-sqlite-corruption-policy.md)). Only a recoverable error
  (`SQLITE_IOERR`, `SQLITE_CORRUPT`, `SQLITE_NOTADB`, or `sqliteutil.ErrUnusableDatabase`, see
  Downgrade) from the read-only probe or from `ensureSchema` deletes the file with its sidecars
  and opens it again. A `SQLITE_BUSY` after
  `busy_timeout` (another writer kept the lock) fails startup and leaves the file untouched.
  Every rebuild leaves empty tables; boot indexing fills them.
- **Which reads detect corruption:** the probe reads the header and `sqlite_master`;
  `ensureSchema` reads `user_version` and `sqlite_master`, and before keeping the tables reads one
  row of every virtual table of the schema. That runs each FTS5 table's constructor, which reads
  its `%_config` shadow table, so a broken FTS5 table behind an intact `sqlite_master` is rebuilt
  too (the `DROP` on every open used to catch this by accident). A full FTS5 `integrity-check` is
  deliberately not run, because its cost grows with the index; corruption deeper in the index
  surfaces as an error when a query or write first reaches it.
- **`Clear` is the healing point:** it drops and recreates every search table (the same
  `recreateSearchSchema`, in a `BEGIN IMMEDIATE` transaction) instead of deleting rows, which
  would keep damaged FTS5 shadow tables. Boot and resync both run `Clear` plus a full reindex,
  so damage the open check cannot see heals at the next start or resync, as it did when every
  open dropped `pages`. Constraint for the fingerprint sync that will replace the boot reindex:
  on a corruption error it must fall back to `Clear` plus a full reindex.
- **Bump rule:** any change to the column set of a search table, its tokenizer options, or the
  text the index writes into it (later: the normaliser output, pinned by a golden test) bumps
  `searchSchemaVersion`. A change to the column set alone is also caught by the DDL check; a
  normaliser change is caught only by the bump.
- **Downgrade:** an older binary that knows `user_version` sees a version different from its own
  and rebuilds; a binary from before this ADR drops `pages` anyway. Either way the next start of
  the newer binary finds a version or DDL mismatch and rebuilds again. Downgrades cost a
  reindex, never a wrongly shaped index. A leftover virtual table whose module the running
  binary lacks cannot be dropped ("no such module"): the rebuild transaction rolls back and
  `ensureSchema` returns `sqliteutil.ErrUnusableDatabase`, so `RebuildOnCorruption` deletes
  the file with its sidecars and the open starts from a fresh one. Today every search table uses
  `fts5`, which every binary has; this matters once a table uses another module.
- **For now boot still reindexes everything:** `IndexAllPagesContext` runs `Clear` and refills
  the index after every open. This ADR only stops the needless `DROP` and adds the mechanism
  that the later steps bump and rely on.
- This is a new pattern for search only. `links.db`, `tags.db` and `properties.db` are out of
  scope and keep their own schema handling. If a second store needs the same check, the
  mechanism gets extracted into a shared seam then (rule 5), not before.

## Alternatives considered

- **Keep dropping on every open**: simple and always consistent, but costs a full reindex and a
  window of partial results on every start, and rules out a boot that trusts persisted rows.
- **`user_version` only**: binaries from before this ADR recreate `pages` without touching
  `user_version`, so a file could carry the current version with an older shape.
- **DDL check only**: does not see a normaliser change, which alters the stored text but not the
  schema.
- **Compare against our DDL string with whitespace normalised**: depends on how SQLite rewrites
  stored `CREATE` statements (it strips `IF NOT EXISTS` and normalises the leading keywords)
  and ignores the FTS5 shadow tables. Creating the schema in memory and comparing SQLite's own text avoids
  both.
- **A metadata table with a version row**: same information as `user_version`, plus one more
  table to keep in sync and to exclude from the schema comparison.
- **Delete the file on mismatch**: would need closing and reopening the connection inside the
  store's open path; dropping tables in one transaction needs no file operations, also on
  Windows.

## Consequences

- search.db keeps its rows across restarts. Until the fingerprint sync lands, boot behaviour
  is the same as before (clear and refill), so users see no difference.
- Every open creates the schema once in an in-memory database; a few milliseconds.
- A `modernc.org/sqlite` update that changes the FTS5 shadow table DDL triggers one rebuild.
  Accepted: it is rare and safe.
- Anyone changing what the index stores has to bump `searchSchemaVersion`; reviewers check that
  for every change to `searchSchema` or to the text written into it.
- Revisit when a second derived store needs the same check (extract a shared seam) or if SQLite
  ever exposes a tokenizer API to this driver, which would move normalisation into the schema.

## Later changes

<!-- Dated entries from the follow-up PRs (page_text and contentless tables, normaliser,
fingerprint sync at boot, trigram table): what changed and which searchSchemaVersion it set. -->

- **2026-11-21, `searchSchemaVersion` 2:** the index replaces char(2) and char(3) in title,
  headings and content with a space before writing them. `highlight()` marks hits with these two
  characters and `sanitizeSearchTitle` turns them into `<b>`/`</b>`, so the same characters in a
  page title faked a highlight in the results (and showed up raw in excerpts).
