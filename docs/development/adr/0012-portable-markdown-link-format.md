# ADR-0012: Portable On-Disk Markdown Link Format

## Status

Proposed (2026-10-05). Implementation started on branch `feat/portable-md-links` (resolution only, no format switch yet).

## Context

[ADR-0001: Filesystem as Source of Truth](./0001-filesystem-source-of-truth.md) makes `root/` the canonical store, but the links written into it only work
inside LeafWiki: page links are absolute routes without extension (`/docs/guide`), assets are `/assets/<pageID>/file`
outside `root/`. Opened in GitHub/GitLab, VS Code or Obsidian, every internal link is dead.

Relative hrefs are resolved with "page is folder" semantics in both backend (`internal/links`) and frontend
(`ui/leafwiki-ui/src/lib/wikiPath.ts`): from `/docs/guide`, `setup` → `/docs/guide/setup`. Markdown tools use file
semantics: from `docs/guide.md`, `setup.md` → `docs/setup.md`. Consequence: GitHub-style links that people (and Claude)
already write by hand into LeafWiki pages are recorded as broken.

## Decision

1. **One canonical on-disk format for Markdown links**: relative paths to the target's Markdown file
   (`../projekte/foo.md`, section → `foo/index.md`). Assets move next to their page (convention still open).
2. **`[[Title]]` wikilinks are a separate, deliberately chosen format and are never rewritten** (rename logic per
   [ADR-0002: Wikilink Sentinel Encoding](./0002-wikilink-sentinel-encoding.md)).
3. **The `.md` suffix selects the semantics.** Hrefs ending in `.md` (ignoring query/fragment, case-insensitive) use file
   semantics relative to the folder holding the page's file (`<route>.md` for pages, `<route>/index.md` for sections);
   all other hrefs keep "page is folder" semantics. Slugs cannot contain dots, so no valid route ends in `.md` — this
   never changes the meaning of a link that resolved before.
4. **Resolution rules live in one pure package** (`internal/core/linkref`) and are pinned by shared test vectors
   (`testdata/link-resolution/vectors.json`) that backend and frontend both run.
5. **The switch to the canonical format is a schema migration, not an option.** No `legacy`/`portable` setting; a
   one-time, idempotent migration rewrites existing links (and moves assets) on upgrade, preceded by an automatic
   snapshot. Restoring an older backup re-runs it via `schema.json`. Parsing keeps accepting every older form so pasted,
   imported or externally edited content is canonicalized on save.
6. **No bidirectional runtime translation.** The editor shows exactly what is on disk.

## Consequences

- Hand-written GitHub-style links work in LeafWiki as soon as point 3 lands, before any format switch.
- The rename/move refactor engine must render `.md` hrefs back in `.md` style; until it does, it leaves them untouched
  rather than converting them to route hrefs.
- Resolving a `.md` href needs the containing page's kind (page vs. section). Converting a page to a section moves its
  file one folder down, so its own relative `.md` links need rewriting on convert.
- The upgrade that switches the format produces one large one-time diff (relevant for git-backed instances); communicated in release notes. No revision per page for the format change.
- Front- and backend resolution can no longer drift silently: any rule change is a vectors-file change.
