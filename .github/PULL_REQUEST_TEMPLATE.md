## Related issue

<!-- Every PR must reference an existing issue. Example: Fixes #123 -->
Fixes #

## What changed

<!-- Short description of the change -->

## How I tested it

<!-- Automated tests added/changed, and anything you checked by hand.
For UI changes a screenshot or short recording is welcome, but not required. -->

## Checklist

- [ ] I discussed the approach on the linked issue before starting (or this is a trivial fix: typo, docs, obvious one-liner)
- [ ] This PR is focused on a single change
- [ ] New behaviour plugs into an extension point; no new `if feature` branches in shared code ([conventions](https://github.com/perber/leafwiki/blob/main/docs/development/conventions.md#extensibility-strict))
- [ ] I ran `npm run format` in `ui/leafwiki-ui` (and in `e2e` if e2e tests changed)
- [ ] I updated documentation if needed
- [ ] For UI changes: I tested it manually in the browser (light/dark mode, mobile width where relevant)
- [ ] AI tools were used for this PR (name them: ___ ). I have reviewed and understand every line
