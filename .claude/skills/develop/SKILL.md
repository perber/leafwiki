---
name: develop
description: Run a LeafWiki change through the full agent workflow — plan, architecture check, implement test-first, independent architecture and code review, independent test, independent security review — with a human gate before implementation (tier feature) and before push. Use for any feature, refactor or non-trivial bug fix; pass the issue number or a description as argument.
---

# Develop a change

Each phase runs as its **own agent with a fresh context** (`.claude/agents/<role>.md`). The
agents that check the work (architecture-reviewer, reviewer, tester, security-reviewer) get only the plan and the
diff, never the implementer's reasoning, so they judge the code and not the intent.

## 0. Pick the tier

The process scales with the change. The planner proposes the tier; the person can raise it,
never lower it.

| Tier | Typical changes | Phases |
|---|---|---|
| **trivial** | typo, rename, comment, docs, dependency bump | implementer → CI checks. No plan gate. |
| **normal** | bug fix or small change inside one slice/feature, no new extension point, no API change | implementer (test-first) → architecture-reviewer + reviewer → tester (edge cases only) |
| **feature** | new feature, new extension point, API or format change, change across several slices | all phases, plan gate |
| **+ security** | anything touching rendering, URLs, uploads, served files, auth, roles, routes, paths, archives — any tier | add security-reviewer |

When unsure between two tiers, take the higher one. Skip the phases a tier doesn't list; don't
skip the ones it does.

## 1. Plan — `planner` (tier feature)

Give the planner the issue/description, then let the `architecture-reviewer` check the plan
(blocking findings → back to the planner). Show the plan to the person and **wait for
approval**. Keep it to one screen; the full detail can follow in the PR description.
Open questions get answered by the person, not guessed. Write the agreed plan where the
project's local overlay says plans go (without one: into the PR description later).

## 2. Implement — `implementer`

One PR-sized step at a time, in an isolated git worktree on its own branch (created from
`origin/main`). A fresh worktree has no `node_modules`: run `npm ci --ignore-scripts` in
`ui/leafwiki-ui` (and `e2e` if needed) before any frontend check; don't symlink another
checkout's `node_modules`, versions may differ. Pass the approved
plan and the step number. The implementer works test-first and reports commands and results.

## 3. Review — `architecture-reviewer` and `reviewer`

Pass both the plan and `git diff <base>...HEAD`. The architecture-reviewer checks SOLID,
Open/Closed, extension points and layering; the reviewer checks correctness, conventions,
tests and size. Blocking findings from either → back to step 2 with the findings list. Repeat
until nothing is blocking.

## 4. Test — `tester`

Pass the plan and the diff. Failures → back to step 2. The tester's manual checklist for UI
changes goes to the person; it holds only what e2e, component or unit tests can't cover.

## 5. Security review — `security-reviewer`

Required when the plan's "Security surface" section says yes, and always for changes under
`ui/leafwiki-ui/src/features/preview/`, file serving, uploads, auth, routes, paths or archives.
Also run the built-in `/security-review` on the same diff and merge both results; the role adds
the LeafWiki-specific checklist, the built-in one a general sweep. Findings `medium` or higher
→ back to step 2.

The checkers in steps 3–5 are independent of each other and can run in parallel once the implementation is
done. After a fix round, rerun every checker whose area the fix touched.

**Different eyes:** the checking roles share the implementer's blind spots when they run on the
same model. Where the tool allows it, run them on a different model (`model:` in the agent
file's frontmatter); keep the implementer on the strongest coding model.

**Tests added by the tester** were not seen by the reviewer (they ran in parallel). Before
hand-over, give the tester's new or changed test files a short reviewer pass, or check them
yourself: exact assertions (one expected status, not "401 or 403"), fixtures reused, right file.

**Findings need evidence.** Only findings with a concrete location and proof (file:line plus
a failing test, a triggering input, or the exact rule violated) can block. Everything else is
a note for the person, not a fix round.

## 6. Hand over

Check the commit history before hand-over: the implementer commits the production change (with
the minimal call-site/test updates it forces) and `test: tidy <area>` separately; tests the
tester added go into their own commit (`test(<area>): …`). Squash fix-round commits into the
commit they fix, so the history reads as clean, separate steps. If a docs file under `docs/development/` states something this change makes
untrue (e.g. a listed exception), fix it in the same PR or note the order dependency with the
docs PR.


Stop after three fix rounds without convergence and ask the person. Otherwise report:
- the diff summary and changed-lines count (code only, ≤ 400–500),
- test, review and security results,
- the manual test checklist for UI changes, limited to what automation can't cover — **a UI
  change with manual items is not done until a person ran them** (no items: say why),
- anything noticed out of scope (as follow-up issues),
- **process feedback:** where the workflow or a rule got in the way, was unclear or missed
  something. These rules are expected to change as we learn; record the observation where the
  local overlay says (without one: in the PR description) so repeated friction turns into a
  rule change.

Local commits are fine; do not push, open a PR or comment on GitHub without the person's explicit OK. After the PR is merged,
remove its worktree (`git worktree remove <path>`), see the `worktree-cleanup` skill.
