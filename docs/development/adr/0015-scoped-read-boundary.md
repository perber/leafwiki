# ADR-0015: Scoped reads go through `internal/scopedread`, not through the core services

## Status

Proposed (2026-10-22)

## Context

Share links (#1252) let someone without an account read one page, or one page and its subtree.
That is the first caller that may see only **part** of the wiki. Until now every reader could
see everything it could reach: a logged-in user, or anyone in public mode. Draft pages (#1478)
and public sections would be the next callers of this kind.

The rules in [conventions](../conventions.md#layers-and-where-code-goes) already say that such
cross-cutting rules (visibility, drafts, access control) do not go into `core/tree` or into
individual handlers, but into one layer above the tree. That layer did not exist yet.

The risks the layer has to address:

- **Leaks through existing reads.** `/api/tree`, `/api/search`, `/api/tags`, `/api/properties`,
  `/api/pages/by-path`, backlinks and the normal page DTO all assume the caller may read
  everything. A scope that has to be remembered in each of them leaks as soon as one is
  forgotten or a new one is added.
- **Leaks through response shapes.** The normal page DTO carries author IDs and resolved author
  names, the slug and path (which reveal the tree above a shared page), and frontmatter data.
- **Probing.** If "out of scope" and "does not exist" answer differently, a scoped caller can
  find out which page IDs exist.
- **Races.** A `*PageNode` returned by `TreeService.FindPageByID` is used after the tree lock is
  released. Walking its `Parent` chain to decide "is this inside the subtree" races with a
  concurrent move.

The approach was agreed with the maintainer on #1252.

## Decision

1. **Core services describe what exists; `internal/scopedread` decides what a scoped caller may
   observe.** Core services (`TreeService`, search, links, tags, properties) do not know about
   scopes, share links or tokens. `internal/architecture_test.go` enforces that `internal/core`,
   `internal/links`, `internal/search`, `internal/tags` and `internal/properties` do not import
   `internal/scopedread`.
2. **`ReadScope{RootID, IncludeDescendants}` is the only thing that crosses the boundary.** It is
   resolved at the feature boundary (a share token is resolved by the sharing feature, never by
   the readers or the core). A scope on the tree root (`"root"`) or on no page is invalid:
   exposing the whole wiki is public mode, not a scope.
3. **Subtree scopes are evaluated against the tree at read time.** Pages created or moved below
   `RootID` later become visible; pages moved out stop being visible.
4. **Readers check the scope before they read content or files.** `PageReader.Page` checks the
   scope first and only then calls `TreeService.GetPage`. Asset readers will do the same before
   touching the filesystem.
5. **Readers return projections with an explicit field allow-list**, never the normal API DTOs
   or core types. `PageView` carries `id`, `title`, `kind`, `content` (the body without
   frontmatter) and `updatedAt`. A test fails when a field is added without updating the
   allow-list.
6. **Out of scope, unknown and deleted are the same `ErrNotFound`.** Operational errors (tree not
   loaded, file read failure) are passed through, so a broken instance is not reported as a
   missing page. They can contain file system paths, so the HTTP layer answers them with a
   generic error and never sends their text to a scoped caller.
7. **Readers depend on small consumer-defined interfaces** (`PageSource`) that the existing
   services already satisfy. The services stay concrete; no interface is introduced for a
   service as a whole.
8. **One structural query was added to the tree:** `TreeService.IsInSubtree(id, rootID)` walks
   the parent chain under the read lock. It answers a question about structure ("is this node
   below that one"), not about visibility, so it stays within what `core/tree` may know.

Abstractions are added with their first use. This ADR's first change adds `ReadScope` and the
single-page reader. The tree reader (navigation inside a subtree scope) and the asset reader
follow with their first callers in later steps of #1252.

## Alternatives considered

- **Pass a scope or a "principal" through the existing read routes** (`APIReadGroup`) and teach
  each handler about it: every existing and future read endpoint becomes a place where a
  forgotten check leaks content. Rejected.
- **Visibility checks inside `TreeService`** (or search, links, tags): turns core into a policy
  engine and couples it to every feature that restricts visibility, against the layer rules.
  Rejected.
- **A generic ACL or policy engine:** much larger than what share links, drafts and public
  sections need; no current requirement for per-user permissions. Out of scope.
- **Walk `PageNode.Parent` outside the tree lock:** a data race with concurrent moves. Rejected
  in favour of `IsInSubtree`.
- **Reuse the normal page DTO and blank out fields:** a field added to the DTO later would leak
  automatically. Rejected in favour of a separate allow-listed projection.

## Consequences

- Scoped features (share links now, possibly drafts and public sections later) get one place to
  read through, and the core services stay unchanged.
- Every new field a scoped caller should see is a deliberate change in this package, guarded by
  a test.
- Subtree scopes cost one parent-chain walk per read (O(depth)) under the tree read lock. Reads
  load the page from disk as before.
- Today a scoped reader loads what it needs from the in-memory tree and filters it. Pushing the
  scope down into store or index queries is left for when profiling shows a need. Scoped search
  is out of scope for #1252; when it comes, the scope has to be applied inside the search query,
  not by filtering results afterwards.
- A page read checks the scope and then reads the page; a page moved out of the scope between
  the two steps can still be served once. This window is the same as for any read racing a move
  and is accepted.
- Revisit this decision if per-user permissions become a requirement; that would need a
  different model than a single root and a subtree flag.
