# AGENTS.md

Instructions for AI coding agents (Claude Code, Codex, Cursor, Copilot, Gemini, …) in this
repository. Humans start at [CONTRIBUTING.md](./CONTRIBUTING.md); the rules are the same.

If `AGENTS.local.md` exists in the repository root, read it next. It holds maintainer-local
instructions, is never committed, and wins where it conflicts with this file.

## The project

LeafWiki is a self-hosted wiki: Markdown files on disk are the source of truth, SQLite indexes
are derived and rebuildable, one binary for Linux, **Windows**, macOS and **Raspberry Pi**.
Backend: layered Go (`cmd/leafwiki` → `internal/wiki` composition root → slices →
`internal/core/*`). Frontend: React SPA in `ui/leafwiki-ui` with feature-local Zustand stores.

Read before changing code — **[docs/development/](./docs/development/README.md)**:
[conventions](./docs/development/conventions.md) (how code is shaped; start with the ten rules
at the top), [source map](./docs/development/source-map.md),
[architecture](./docs/development/architecture.md),
[processes](./docs/development/processes.md), [ADRs](./docs/development/adr/README.md).

## Commands

| What | Command | Where |
|---|---|---|
| Backend tests / lint | `go test ./...` / `golangci-lint run` (v2) | repo root |
| Windows build check | `GOOS=windows go build ./...` | repo root |
| Frontend typecheck + build | `npm run build` (not `npx tsc --noEmit`, it checks nothing) | `ui/leafwiki-ui` |
| Frontend tests / lint / format | `npm test` / `npm run lint -- --max-warnings=0` / `npm run format` | `ui/leafwiki-ui` |
| E2E | `make run-e2e`; lint/format in `e2e` | repo root |
| Local dev | backend: `go run . --jwt-secret=dev --admin-password=dev --public-access=true --allow-insecure=true` in `cmd/leafwiki`; frontend: `npm run dev` | |

A fresh worktree has no `node_modules`: run `npm ci --ignore-scripts` in `ui/leafwiki-ui` first.

## Rules

1. **Keep the architecture open — SOLID, Open/Closed first.** New features plug into an
   extension point; shared code (composition root, router, `TreeService`, `MarkdownPreview`, …)
   registers them and never grows `if feature` branches. No fitting seam → add it first, in its
   own PR. Cross-cutting rules (visibility, drafts, access control) go into one layer above the
   tree, never into `core/tree`. Read two neighbouring files and copy their shape.
2. **Plan first when scope is unclear.** Files, risks, implied refactoring, cross-platform
   impact. Architecture decisions get an ADR.
3. **Tests first.** New behaviour needs a test; a bug gets a failing regression test before the
   fix. Leave the tests you touch better (separate `test: tidy …` commit, no lost case).
4. **Automate UI tests; a manual click test is required only where automation can't reach.**
   Behaviour gets e2e, component or unit tests. A manual checklist covers only the rest (visual
   look, light/dark, mobile width, real-browser specifics); UI work with such items is not done
   until someone has run them. Clicking through by hand on top is always welcome.
5. **Stay in scope.** Notice a bug elsewhere → report it, don't fix it silently. Keep
   `docs/`, `README.md` and `.env.example` in sync with what you change.
6. **Comments explain why** and reference only GitHub issues or ADRs — never private notes,
   local paths or chat history.
7. Everything else — errors, API, i18n, naming, reuse, dependencies, migrations, deprecation,
   feature flags, concurrency, cross-platform — is in the
   [conventions](./docs/development/conventions.md). Much of it is enforced in CI (import
   boundaries, ESLint XSS guard, i18n key parity, golangci-lint); a red check is a real finding.

## Commits and pull requests

- **Committing locally is fine** when the history is clean: one logical change per commit,
  Conventional Commits (`type(scope): description`), refactoring, behaviour change and
  `test: tidy` in separate commits, formatters run, tests green. Never commit secrets,
  generated files or local overlays.
- **Pushing, opening PRs and commenting on GitHub need the person's explicit OK.**
- PRs: ≤ 400–500 lines of production code (tests and docs don't count), one topic, `main`
  always works (unfinished work behind a default-off flag), README changes in a separate PR at
  release time. See [CONTRIBUTING.md](./CONTRIBUTING.md#pull-request-guidelines) and
  [AI-assisted contributions](./CONTRIBUTING.md#ai-assisted-contributions).

## Workflow and roles

Changes run through independent roles, scaled by tier (details in the `develop` skill):
**trivial** (typo, rename, docs) → implement + CI; **normal** (change inside one slice) →
implement + architecture-reviewer + reviewer + tester; **feature** (new feature, extension
point, API or format change) → all roles with a plan gate; **+ security** for anything touching
rendering, uploads, served files, auth, routes or paths.

Roles in `.claude/agents/` (plain Markdown prompts): `planner`, `architecture-reviewer`,
`implementer`, `reviewer`, `tester`, `security-reviewer`. Each runs with a fresh context; the
checking roles see only the plan and the diff. Without subagent support, run each role in its
own session with its file as instructions.

## Skills

Runbooks in `.claude/skills/<name>/SKILL.md`: `develop` (the workflow above), `tidy-tests`
(restructure a package's tests with proof nothing was lost), `run` (start backend + frontend),
`docs-review` (docs vs. code), `worktree-cleanup`.
