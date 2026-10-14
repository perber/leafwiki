@AGENTS.md

## Claude Code specifics

- `AGENTS.md` above is the canonical, tool-neutral instruction file. Put new project rules there,
  not here; this file only holds what is specific to Claude Code.
- Maintainer-local instructions live in `AGENTS.local.md` (gitignored). A `CLAUDE.local.md`
  containing `@AGENTS.local.md` makes Claude Code load it automatically.
- For an unclear task, use Plan mode before editing (see "How to work" in `AGENTS.md`).
- Skills in `.claude/skills/` are listed in `AGENTS.md`. Skill folders that are gitignored are
  maintainer-only.
