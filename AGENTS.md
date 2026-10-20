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
[rules](./docs/development/rules.md) (the short list, same for people and agents),
[conventions](./docs/development/conventions.md) (the reference behind them),
[source map](./docs/development/source-map.md), [architecture](./docs/development/architecture.md),
[processes](./docs/development/processes.md), [ADRs](./docs/development/adr/README.md).

## Commands

| What | Command | Where |
|---|---|---|
| Backend tests / lint | `go test ./...` / `golangci-lint run` (v2) | repo root |
| Repository checks (private references) | `make verify` | repo root |
| Windows/macOS build check | `make cross` (build + vet for windows/amd64, darwin/arm64) | repo root |
| Frontend typecheck + build | `npm run build` (not `npx tsc --noEmit`, it checks nothing) | `ui/leafwiki-ui` |
| Frontend tests / lint / format | `npm test` / `npm run lint -- --max-warnings=0` / `npm run format` | `ui/leafwiki-ui` |
| E2E | `make run-e2e`; lint/format in `e2e` | repo root |
| Local dev | backend: `go run . --jwt-secret=dev --admin-password=dev --public-access=true --allow-insecure=true` in `cmd/leafwiki`; frontend: `npm run dev` | |

A fresh worktree has no `node_modules`: run `npm ci --ignore-scripts` in `ui/leafwiki-ui` first.

## Rules

The rules live in **[docs/development/rules.md](./docs/development/rules.md)**, nowhere else.
Read them before changing code; don't restate them in prompts, skills or roles, link there.
Much of it is enforced in CI (import boundaries, ESLint XSS guard, i18n key parity,
golangci-lint, Prettier); a red check is a real finding.

## Commits and pull requests

- **Committing locally is fine** when the history is clean: one logical change per commit,
  Conventional Commits (`type(scope): description`), refactoring, behaviour change and
  `test: tidy` in separate commits, formatters run, tests green. Never commit secrets,
  generated files or local overlays.
- **Pushing, opening PRs and commenting on GitHub need the person's explicit OK.**
- PR size and scope: [rules](./docs/development/rules.md) and
  [conventions](./docs/development/conventions.md#pull-requests); process in
  [CONTRIBUTING.md](./CONTRIBUTING.md#pull-request-guidelines) and
  [AI-assisted contributions](./CONTRIBUTING.md#ai-assisted-contributions).

## Workflow and roles

Changes run through independent roles, scaled by tier (details in the `develop` skill):
**trivial** (typo, rename, docs) → implement + CI; **normal** (change inside one slice) →
implement + tester + architecture-reviewer + reviewer; **feature** (new feature, extension
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
