# ADR-0006: Frontend state is per-feature Zustand stores that call each other directly

## Status

Accepted (retroactively documented, 2026-07-07). Partially superseded in intent — a TanStack Query migration has been discussed as the next step for the data-fetching half of this pattern, but is not started. This ADR should be revisited if/when that migration begins.

## Context

The frontend needs both UI state (dialog open/closed, sidebar width, editor preferences) and server state (page content, tree, users, config) across ~20 feature folders. No Redux was in use; the question was how to structure state for both concerns without pulling in a large framework.

## Decision

Use Zustand (v5) uniformly for both UI state and server state. Global, app-wide concerns live in `src/stores/` (tree, session, config, dialogs, sidebar). Feature-specific state is co-located inside `features/<name>/` (`pageEditorStore`, `viewerStore`, `progressbarStore`, etc.) rather than centralized.

Cross-feature effects are handled by stores calling each other's `getState()` directly — e.g. `pageEditorStore.savePage()` reaches into `useTreeStore`, `useViewerStore`, `useConfigStore`, `useProgressbarStore`, and `useLinkStatusStore` — rather than routing orchestration through components or a shared coordination layer.

## Consequences

- Low ceremony, no extra framework dependency, easy to find a feature's state (it lives next to the feature).
- Server-state concerns (loading/error/cancellation) are hand-rolled per store with no shared caching, invalidation, or dedup layer — ~15 stores repeat near-identical `isLoading`/`error`/`AbortController` plumbing. This is the single largest structural duplication in the frontend.
- The "stores call stores" pattern makes cross-feature coupling invisible at the component level — reading `PageEditor.tsx` doesn't tell you it also touches the tree, viewer, and link-status stores; you have to read `pageEditorStore.ts` itself. Anyone refactoring one of the involved stores needs to grep for `getState()` call sites in the others, not just look at component usage.
- A future TanStack Query migration would primarily replace the server-state half of this pattern (fetching/caching/invalidation), not necessarily the UI-state half or the stores-call-stores convention — worth deciding explicitly, when that migration starts, whether the cross-store call pattern is kept or replaced with query invalidation.
