---
name: tidy-tests
description: Restructure the tests of one LeafWiki package, file or feature for readability — split oversized test files by capability, turn copy-pasted cases into table-driven (matrix) tests, introduce a default fixture with overrides — without changing what is tested. Produces proof that no case and no coverage was lost. Use for dedicated test-cleanup PRs (e.g. "split router_test.go", "tidy the pages tests"); pass the package or file as argument.
---

# Tidy tests

A behaviour-preserving refactoring of test code only. The rules are in
`docs/development/conventions.md` ("Tests"). Production code is not touched — if a test can
only be simplified by changing production code, note it and stop there.

## 1. Baseline (before any edit)

Record, for the target package(s):

```bash
go test ./<pkg>/... -count=1 -v 2>&1 | grep -E '^(=== RUN|--- (PASS|FAIL|SKIP))' > before-cases.txt
go test ./<pkg>/... -count=1 -coverprofile=before.cov && go tool cover -func=before.cov | tail -1
```

Frontend: `npx vitest run <path> --reporter=verbose > before-cases.txt` and
`npm run test:coverage -- <path>`. Keep these files outside the repo (scratch directory).

## 2. Plan the restructuring

List what you will change and why, grouped:

- **Split** a file over ~800 lines by capability (e.g. `router_test.go` → `router_pages_test.go`,
  `router_auth_test.go`, …, shared setup in `router_fixture_test.go`).
- **Tables:** groups of tests with the same setup and assertion that differ only in input and
  expected output. Name every row.
- **Fixture:** one default fixture per package with explicit overrides; `t.Cleanup` for every
  opened store; build only what each test needs.
- **Names** to `Test<Type>_<Scenario>_<Expectation>`; dead helpers removed.

Don't table cases with different setup or different assertions. No generic assertion DSL.
Show the plan to the person if it touches more than one package.

## 3. Restructure in small steps

One kind of change per step, each meant to become its own commit (`test: split router tests by slice`, `test: table-drive slug
validation`, …), the suite green after each. Commit each step locally (Conventional Commits, suite green before each);
don't push. The PR is capped at ~1,000 changed lines; a big
file is split over several PRs.

## 4. Prove nothing was lost

```bash
go test ./<pkg>/... -count=1 -v 2>&1 | grep -E '^(=== RUN|--- (PASS|FAIL|SKIP))' > after-cases.txt
go test ./<pkg>/... -count=1 -coverprofile=after.cov && go tool cover -func=after.cov | tail -1
```

- **Coverage** after ≥ before (per package).
- **Cases:** map every test from `before-cases.txt` to a test or table row in the new code. Old
  test functions that became rows have a different name; the mapping goes into the PR
  description as a table (old name → new location). A case without a mapping is a lost case:
  restore it.
- No assertion weakened (e.g. exact comparison replaced by "not empty"), no `t.Skip` added.
- Lint and format: `golangci-lint run ./<pkg>/...`; frontend `npm run lint`, `npm run format`.

## 5. Review

Run the `reviewer` agent on the diff with the before/after case lists and coverage numbers.
A lost case or lower coverage is blocking. Don't push or open a PR without the person's OK.
