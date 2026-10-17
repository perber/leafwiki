---
name: worktree-cleanup
description: List this repository's git worktrees with their branch state (merged, PR open, uncommitted changes, age) and remove the ones that are done — only after the person confirms. Use after a PR is merged, or when the person says "clean up the worktrees".
---

# Worktree cleanup

Agent work creates one worktree per branch; they pile up. Never delete anything without
showing the list and getting an explicit OK.

## 1. Collect

```bash
git fetch -q --prune origin
git worktree list --porcelain
```

For every worktree except the main checkout, determine:

- **branch** (or detached HEAD) and last commit date (`git -C <path> log -1 --format=%cr`),
- **uncommitted changes**: `git -C <path> status --porcelain` (count tracked + untracked),
- **merged**: `git branch --merged origin/main` contains the branch, or `gh pr list --state
  merged --head <branch>` finds a merged PR (squash merges don't show up as merged in git),
- **open PR**: `gh pr list --state open --head <branch>`,
- **prunable**: marked `prunable` by git (directory already gone).

## 2. Propose

Show a table grouped as:

1. **Safe to remove** — merged (or PR merged), no uncommitted changes.
2. **Probably done** — no open PR, last commit older than 30 days, no uncommitted changes.
3. **Keep** — open PR, uncommitted changes, or recent work. List why.

## 3. Remove (after OK, per group or per entry)

```bash
git worktree remove <path>          # refuses if there are uncommitted changes — good
git branch -d <branch>              # only for merged branches; -D only if the person says so
git worktree prune
```

Never use `--force` on a worktree with uncommitted changes unless the person explicitly asked
for that specific one. Report what was removed and what was kept.
