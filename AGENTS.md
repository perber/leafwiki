# AGENTS.md

Instructions for AI coding agents (Claude Code, Codex, Cursor, Copilot, Gemini, …) working in
this repository. Human contributors: [CONTRIBUTING.md](./CONTRIBUTING.md) is your entry point.
Everything here applies to humans too; it is just written to be followed literally.

## Local overlay

If `AGENTS.local.md` exists in the repository root, read it right after this file. It holds
maintainer-local instructions (private issue tracker, local paths, personal workflow) and is
never committed. Where it conflicts with this file, it wins.

## What LeafWiki is

A self-hosted wiki: Markdown files on disk, single binary, minimal operational complexity.
Project principles (keep them intact, see [CONTRIBUTING.md](./CONTRIBUTING.md#project-principles)):

- Markdown stored on disk is the single source of truth. The SQLite indexes for links, tags,
  properties and search are derived and must always be rebuildable
  ([ADR-0001](./docs/development/adr/0001-filesystem-source-of-truth.md)).
- Single binary deployment, self-hosting friendly, explicit structure over hidden automation.
- Ships for Linux (amd64, 386, arm64), **Windows**, macOS and **Raspberry Pi**.

## Where to look

- [docs/development/source-map.md](./docs/development/source-map.md): what every backend and
  frontend directory does.
- [docs/development/architecture.md](./docs/development/architecture.md): layering, storage
  model, auth model.
- [docs/development/processes.md](./docs/development/processes.md): how core workflows run end
  to end (page save, revisioning, wikilink resolution, resync, editor auto-save, tree
  rename/move, auth).
- [docs/development/adr/](./docs/development/adr/): architecture decision records. Check them
  before changing something that looks like an odd choice; it may be a deliberate, documented
  tradeoff.
- [docs/i18n.md](./docs/i18n.md): UI translations.

Short version: the backend is a layered Go app (`cmd/leafwiki` → `internal/wiki` composition
root → domain slices → `internal/core/*`). The frontend (`ui/leafwiki-ui`) is a React SPA with
one feature-local Zustand store per domain
([ADR-0006](./docs/development/adr/0006-zustand-feature-local-stores.md)).

## Commands

| What | Command | Where |
|---|---|---|
| Backend tests | `go test ./...` | repo root |
| Backend lint | `golangci-lint run` (v2, config in `.golangci.yml`) | repo root |
| Frontend dev server | `npm run dev` | `ui/leafwiki-ui` |
| Frontend unit tests | `npm test` | `ui/leafwiki-ui` |
| Frontend lint | `npm run lint -- --max-warnings=0` | `ui/leafwiki-ui` |
| Frontend typecheck + build | `npm run build` | `ui/leafwiki-ui` |
| Frontend format | `npm run format` | `ui/leafwiki-ui` |
| E2E tests (Playwright) | `make run-e2e` | repo root |
| E2E lint / format | `npm run lint`, `npm run format` | `e2e` |
| Backend for local dev | `go run . --jwt-secret=dev --admin-password=dev --public-access=true --allow-insecure=true` | `cmd/leafwiki` |

Typecheck the frontend with `npm run build` (`tsc -b`), not `npx tsc --noEmit`: the root
tsconfig only references the app configs and checks nothing by itself.

## How to work

- **Unclear scope → plan first.** If the task's scope or approach isn't clear, write a plan
  before touching code and get it agreed. The plan names the files to change, risks and
  tradeoffs, any refactoring it implies (call it out, don't bundle it in silently), and
  cross-platform impact.
- **Tests are required. Bugs get a regression test first.** New behavior needs a test. For a
  bug fix, first write a test that reproduces the bug and confirm it fails, then fix it.
- **UI changes need a human to test them.** Automated tests are not enough for anything a user
  sees. When you change UI, end your work with a concrete manual test checklist (pages, clicks,
  expected result, light/dark mode, mobile width where relevant) for a person to run in a
  browser. Don't call a UI change done until a person has confirmed it.
- **Organize tests well and leave them better than you found them.** Same-shape cases go into
  a table (`t.Run` / `it.each`). When you touch a test file, tidy the tests around your change
  in a separate `test: tidy …` commit — behaviour-preserving, no lost case, coverage not lower.
  Details: [conventions: Tests](./docs/development/conventions.md#tests).
- **Decide on logging while implementing, not afterwards.** For state changes, recoverable
  fallbacks and swallowed errors, decide whether a log line is needed. Follow
  [ADR-0008](./docs/development/adr/0008-structured-logging-schema.md) for levels, field names
  and the "log once, at the swallow point" rule.
- **Keep the architecture extensible — strictly (SOLID, above all Open/Closed: open for
  extension, closed for modification).** New features plug into an existing extension
  point (route registrar, page-save side effects, providers, frontend registries). If none fits,
  add the extension point first in its own PR. Shared code (the composition root `internal/wiki/wiki*.go`, router, `TreeService`,
  `MarkdownPreview`, …) registers features, it never grows `if feature` branches. Ask yourself:
  could a second feature of this kind be added without touching this code again? Details:
  [conventions: Extensibility](./docs/development/conventions.md#extensibility-strict).
- **Extract what is used more than once** — the same logic a second time is a smell, a third
  copy is not allowed. Where it goes depends on its dependencies (package helper → domain
  package → `internal/core/shared/<topic>` / `lib/` / `components/`), see
  [conventions: Reuse](./docs/development/conventions.md#reuse-extract-what-is-used-more-than-once).
- **Follow [docs/development/conventions.md](./docs/development/conventions.md)** for layering
  (what goes into `internal/core`), file layout and order, use-case shape, error codes, HTTP API
  and frontend structure. Before writing new code in an area, read two neighbouring files and
  copy their shape.
- **Architecture decisions get an ADR.** See
  [docs/development/adr/README.md](./docs/development/adr/README.md) for when one is required
  and the template.
- **Stay in scope.** Bugs, gaps or follow-ups you notice outside the task: don't fix them
  silently. Note them down (where depends on the local overlay; without one, list them in your
  final summary so a person can file a GitHub issue).
- **Keep docs in sync.** If you change a flag, env var, default or user-visible behavior,
  update `README.md`, `docs/` and `.env.example` in the same change. If you learn how something
  actually works and `docs/development/` says otherwise, fix the doc. Docs name code by package,
  file and function, never by line number.
- **Comments explain why, not what.** Reference GitHub issues (`#123`) or ADRs, never private
  trackers, local paths or chat history; a reader of the public repo must be able to follow
  every reference.
- **Don't commit, push, open PRs or comment on GitHub without the person's explicit OK.**

## Workflow: plan, implement, review, test, security

Changes go through up to six roles, scaled by tier — **trivial** (typo, rename, docs: implement
+ CI), **normal** (fix inside one slice: implement test-first + architecture-reviewer + reviewer
+ tester), **feature** (new feature, extension point or API change: all roles with a plan
gate), plus the security-reviewer for anything touching rendering, uploads, auth, routes or
paths. The roles are each an **independent agent with a fresh context**.
The checking roles get only the plan and the diff, not the implementer's reasoning.

| Role | Does | Edits code? |
|---|---|---|
| planner | scope, extension point, PR series, tests, risks, security surface | no |
| implementer | one PR-sized step, test-first | yes |
| architecture-reviewer | SOLID with Open/Closed first, extension points, layering — on the plan and on the diff | no |
| reviewer | correctness, conventions, tests, PR size | no |
| tester | runs all suites, probes edge cases, manual UI checklist | tests only |
| security-reviewer | XSS (rendering, URLs, raw DOM, served files) and backend (authz, CSRF, paths, archives, secrets) | no |

A person approves the plan before implementation and the result before commit; UI changes
also need the person's manual test. Role definitions: `.claude/agents/<role>.md` (plain
Markdown prompts). Claude Code runs them as subagents via the `develop` skill; with other
tools, start each role in a new session with that file as instructions.

## Code conventions

### Backend (Go)

- Domain errors are `*sharederrors.LocalizedError` (`internal/core/shared/errors`), not
  `errors.New` / `fmt.Errorf`. `internal/backup` is a known exception; don't copy it.
- Tests: stdlib `testing` only, no testify. Names follow `Test<Type>_<Scenario>_<Expectation>`.
  Prefer real-filesystem integration tests via `t.TempDir()` over mocks.
- Ignored close/cleanup errors are explicit: `defer func() { _ = f.Close() }()`.
- No new dependencies without discussing it first (criteria:
  [conventions: Dependencies](./docs/development/conventions.md#dependencies)).
- On-disk format changes need a schema bump, an idempotent `treemigration` step with a test from
  the old layout, and an ADR; nothing users depend on is removed without a one-minor deprecation
  period. Names of flags, env vars, metrics, frontmatter keys and branches follow
  [conventions: Naming](./docs/development/conventions.md#naming).
- Goroutines are owned and stopped by the shutdown context; `ctx` is passed down; no I/O under
  the tree lock.

### Frontend (React / TypeScript)

- UI strings go through `react-i18next` (`useTranslation('<namespace>')`, one namespace per
  feature in `ui/leafwiki-ui/src/locales/<lang>/`). Never hardcode them, never build sentences
  from pieces (interpolation), plurals via `count`, markup via `<Trans>`, dates via
  `useDateTimeFormat`. Rules: [conventions: i18n](./docs/development/conventions.md#internationalization-i18n).
  If you touch a component that still has hardcoded strings, migrate it.
- **Key parity:** `src/lib/i18n.test.ts` discovers every locale directory and requires
  identical keys in all of them. A new key or namespace file must be added to **every** locale,
  or the suite fails.
- Use the shadcn/Radix primitives in `ui/leafwiki-ui/src/components/ui/` (`Input`, `Select`,
  `Checkbox`, `Dialog`, …) instead of raw `<input>`, `<select>` etc. If a primitive is
  missing, add it there. Reuse existing components and CSS conventions before building new
  ones.
- Tests: Vitest + Testing Library for units, Playwright in `e2e/` for end to end.

### Cross-platform

- Use `filepath` for OS paths and `path` for URL/slug paths; never hardcode `/` or `\`.
- Don't assume a case-sensitive filesystem (Windows and macOS aren't).
- Keep memory and CPU use reasonable for a Raspberry Pi.
- OS-specific code goes into build-tagged files (see `cmd/leafwiki/signals_unix.go` vs
  `signals_windows.go`).

## Commits and pull requests

- **At most 400–500 changed lines of production code per PR** (tests, docs, lockfiles,
  generated files, pure moves and non-English locale files don't count; soft limit ~1,000 lines
  in total). Split bigger work into a series; each PR must leave `main` working, with
  unfinished features behind a default-off feature flag. README changes go in a separate PR
  merged at release time. Details: [conventions](./docs/development/conventions.md#pull-requests).

- Conventional Commits: `type(scope): short description` (`feat`, `fix`, `docs`, `refactor`,
  `chore`, `test`).
- Run `npm run format` in `ui/leafwiki-ui` (and in `e2e` if e2e tests changed) before
  committing.
- One focused change per PR, linked to an issue. Follow
  [CONTRIBUTING.md](./CONTRIBUTING.md#pull-request-guidelines) and the
  [AI-assisted contributions](./CONTRIBUTING.md#ai-assisted-contributions) rules.

## Skills

Reusable step-by-step workflows live in `.claude/skills/<name>/SKILL.md` (the open Agent Skills
format). Claude Code loads them automatically; other agents can read the `SKILL.md` and follow
it as a runbook.

- `run`: start backend and frontend locally to verify a change in the browser.
- `tidy-tests`: behaviour-preserving restructuring of one package's tests (split big files,
  table-driven cases, shared fixture) with before/after proof of cases and coverage.
- `worktree-cleanup`: list stale git worktrees and remove merged ones after confirmation.
- `develop`: run a change through planner → architecture-reviewer → implementer → architecture-reviewer / reviewer / tester / security-reviewer.
- `docs-review`: check README, `docs/` and `.env.example` against the code.
