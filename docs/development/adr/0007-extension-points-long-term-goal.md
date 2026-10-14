# ADR-0007: Long-term goal — explicit extension points for backend and frontend

## Status

Accepted (statement of direction, 2026-07-07). This records an intent, not a finished design — there is no plugin API, extension manifest, or hook registry implemented yet. Treat this as a constraint on *future* architecture decisions, not a spec to build against today.

## Context

LeafWiki's principles favor "explicit structure over hidden automation" and a single-binary, self-hosting-friendly deployment. Neither of those principles rules out extensibility, but nothing in the codebase today is designed to be *hooked into* from outside its own package: the backend has no plugin/module loading mechanism, and the frontend has no registration API for third-party UI or routes. Any customization currently means forking.

As the project grows (see community requests such as the "Sharing Sections" feature request), it's expected that some customization needs won't belong in core — either because they're niche, deployment-specific, or because they'd otherwise bloat the single binary. Extension points are a way to keep the core small while still allowing those needs to be met.

## Decision

LeafWiki should, over time, grow well-defined extension points on both sides of the stack — places where external or optional code can hook into core behavior without modifying core packages:

- **Backend**: natural candidate anchor points already exist structurally, e.g. the `internal/wiki/pagesave` side-effect bus (already the single place that knows about all post-save side effects — revision, links, tags, properties, search) and the composition root in `internal/wiki/wiki.go`. A hook/registration mechanism at these seams (rather than a full dynamic-plugin-loading system) is the likely shape, to stay consistent with "single binary" and "minimal operational complexity."
- **Frontend**: the per-feature Zustand store structure and route table (`features/router/router.tsx` → `lazy-routes.tsx`) are the natural seam for a future UI extension mechanism (e.g. registering additional routes, sidebar entries, or editor panels) — see [ADR-0006: Zustand Feature-Local Stores](./0006-zustand-feature-local-stores.md) for the store pattern this would build on.

No specific API, manifest format, sandboxing model, or versioning scheme is decided by this ADR. The decision is scoped to: extension points are a deliberate long-term goal, and future refactors of the pagesave bus, the composition root, and the frontend route/store structure should be evaluated partly on whether they make a future extension mechanism easier or harder to introduce — not just on their immediate simplification value.

## Consequences

- When refactoring the composition root in `internal/wiki` (since split into `wiki.go`, `wiki_boot.go`, `wiki_routes.go`, `wiki_resync.go`) or the `pagesave` side-effect bus, prefer shapes that could later accept externally-registered hooks (e.g. an ordered list of side-effect handlers) over ones that hard-code the exact set of steps inline.
- When touching the frontend route table or store composition, avoid decisions that would make "register an additional feature from outside `src/features/`" structurally impossible later (e.g. assuming all routes are statically known at one call site with no extension seam).
- This is explicitly *not* a commitment to ship a plugin system soon — no timeline, no issue tracking this yet. It exists so that near-term refactors don't quietly close off the option.
- Self-hosting-friendliness and single-binary deployment remain the stronger, near-term constraints; if a proposed extension mechanism would require dynamic loading, a package manager, or network-fetched code at runtime, that conflicts with those principles and should be revisited against them explicitly when the time comes.

## Related future idea (not scoped, not scheduled)

Git Backup (`internal/backup`, see the README's "Git Backup" section) currently resolves remote divergence by letting the local backup repo win — `ForcePush` overwrites the remote wholesale, with no merge. That's a reasonable default for a single self-hosted instance, but it gets weaker once a team also touches the same remote through their own Git tooling outside the app (discussed 2026-07, prompted by a README pass on external-edit/resync behavior). A real extension of this area, if the project keeps growing, would be a proper 3-way-merge attempt before falling back to forced overwrite, a diff preview before force-pushing, and eventually true bidirectional sync (external Git changes flowing back into wiki content, not just wiki content flowing out). None of this is designed or scheduled — noted here only so it isn't lost, and so any future rework of `internal/backup` can keep the door open the same way the pagesave bus and route table are meant to.
