@AGENTS.md
@docs/development/rules.md

## Claude Code specifics

- `AGENTS.md` above is the canonical, tool-neutral instruction file; the rules it points to are
  in `docs/development/rules.md` (imported above). Put new project rules there, not here; this
  file only holds what is specific to Claude Code.
- Maintainer-local instructions live in `AGENTS.local.md` (gitignored). A `CLAUDE.local.md`
  containing `@AGENTS.local.md` makes Claude Code load it automatically.
- For an unclear task, use Plan mode before editing (rule 12 in `docs/development/rules.md`).
- Skills in `.claude/skills/` and roles in `.claude/agents/` are listed in `AGENTS.md`; the
  `develop` skill runs the roles as subagents. Untracked skill folders are maintainer-only.
