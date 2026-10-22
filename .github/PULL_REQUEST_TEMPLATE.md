## Related issue

<!-- Every PR must reference an existing issue. Example: Fixes #123 -->
Fixes #

## What changed

<!-- Short description of the change -->

## How I tested it

<!-- Automated tests added/changed (unit, component, e2e). Prefer automated tests for behaviour;
list here only what you checked by hand because it can't be automated (visual look, light/dark,
mobile width, real-browser specifics) and why. Extra manual testing is always welcome.
For UI changes a screenshot or short recording speeds up review, but is not required. -->

## Checklist

- [ ] I discussed the approach on the linked issue before starting (or this is a trivial fix: typo, docs, obvious one-liner)
- [ ] This PR is focused on a single change
- [ ] New behaviour plugs into an extension point; no new `if feature` branches in shared code ([rule 1](https://github.com/perber/leafwiki/blob/main/docs/development/rules.md#architecture))
- [ ] I updated documentation if needed
- [ ] If users or admins will notice a change in behaviour, the commit carries a `Behavior-Change:` trailer ([commit style](https://github.com/perber/leafwiki/blob/main/CONTRIBUTING.md#commit-style))
- [ ] For UI changes: behaviour is covered by automated tests where possible ([rule 11](https://github.com/perber/leafwiki/blob/main/docs/development/rules.md#tests))
- [ ] For UI changes: I checked by hand what tests can't cover (look in light/dark mode, mobile width where relevant)
- [ ] AI tools were used for this PR (name them: ___ ). I have reviewed and understand every line
