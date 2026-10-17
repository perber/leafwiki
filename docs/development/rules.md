# Rules

The rules that matter most when changing LeafWiki, for people and coding agents alike. This is
the only place they are written down; `AGENTS.md`, `CONTRIBUTING.md`, the PR template and the
agent roles link here instead of restating them. Details and exceptions are in the
[conventions](./conventions.md); look up the section for the area you are working in.

A rule that a check enforces says so. Where the check and this text disagree, the check wins
and this page gets fixed.

## Architecture

1. **Open/Closed.** New features plug into an extension point; shared code (composition root,
   router, `TreeService`, `MarkdownPreview`, …) registers them and never grows `if feature`
   branches. No fitting seam → add the seam first, in its own PR.
   [Extensibility](./conventions.md#extensibility-strict)
2. **Layers.** Dependencies point down the layer table.
   [Layers](./conventions.md#layers-and-where-code-goes) · *Enforced by
   `internal/architecture_test.go`.*
3. **Cross-cutting rules** (visibility, drafts, access control) live in one layer above the
   tree, never in `core/tree` or single handlers.
4. **Copy the neighbours.** Read two neighbouring files in the same package and copy their
   shape. A new pattern or an architecture decision needs an [ADR](./adr/README.md).
5. **Reuse.** The same logic a second time gets extracted, placed by its dependencies.
   [Reuse](./conventions.md#reuse-extract-what-is-used-more-than-once)

## Code

6. **Use cases** take `Execute(ctx, in XxxInput)`; domain errors are `LocalizedError` with
   `<domain>_<reason>` codes. [Use case shape](./conventions.md#use-case-shape) ·
   [Errors](./conventions.md#errors)
7. **Every UI string through i18n**, in every locale; form controls are shadcn primitives.
   [i18n](./conventions.md#internationalization-i18n) · *Enforced by `i18n.test.ts`.*
8. **No raw HTML sinks** (`innerHTML`, `dangerouslySetInnerHTML`, …). *Enforced by ESLint.*
9. **Comments explain why** and reference only GitHub issues or ADRs, never private notes,
   local paths or chat history.

## Tests

10. **Tests first.** New behaviour needs a test; a bug gets a failing regression test before
    the fix. Leave the tests you touch better, in a separate `test: tidy …` commit, without
    losing a case. [Tests](./conventions.md#tests)
11. **UI behaviour gets automated tests** (e2e, component or unit); a person checks in the
    browser what automation can't reach (look, light/dark, mobile width, browser quirks), and
    the change isn't done until someone has.

## Scope and pull requests

12. **Plan first when scope is unclear:** files, risks, implied refactoring, cross-platform
    impact.
13. **Stay in scope.** A bug noticed elsewhere is reported, not fixed silently. Keep `docs/`,
    `README.md` and `.env.example` in sync with what you change.
14. **PRs:** ≤ 400–500 lines of production code, one topic, `main` always works (unfinished
    work behind a default-off flag). [Pull requests](./conventions.md#pull-requests)
