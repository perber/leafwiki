---
name: docs-review
description: Review LeafWiki's documentation (README.md, docs/, CONTRIBUTING.md, .env.example) against the actual code — flags/env vars/defaults vs `leafwiki --help`, broken links and anchors, stale language lists, UI labels the docs mention, and features that exist in frontend/backend but are documented nowhere. Reports findings and proposes fixes; asks before documenting new features. Use when the user wants to "go over the docs", "check the documentation", before a minor release (the `release` skill calls it), or after adding/renaming flags.
---

# Docs review

Docs live in the repo, not on a separate site (decided 2026-10-09): `README.md` is a short entry
point (~170 lines), everything else is under `docs/` (see `docs/README.md` for the index). Docs
change in the same PR as the code they describe.

Layout:

| Path | Holds |
|---|---|
| `README.md` | intro, quick start, features, good fit, install table, access modes, docs index, support, contributing |
| `docs/install/` | docker, kubernetes, linux-installer, binary, nginx, raspberry |
| `docs/configuration/reference.md` | **every** flag + env var + default + since, grouped like `--help` |
| `docs/configuration/*.md` | access-modes, reverse-proxy-auth, git-backup, email, unix-socket, customization |
| `docs/operations/` | admin-commands (reset password, offline restore, defaults), resync |
| `docs/usage/` | keyboard-shortcuts, links, sorting-pages |
| `docs/development/` | dev-setup, i18n, architecture, source-map, processes, `adr/` |

Rules: a flag's table row exists only in `reference.md`; topic pages explain and link there instead
of repeating tables. No hand-written table of contents in README. Work in a separate worktree when
another session might be using the main checkout.

## 1. Automated checks (run all, from the repo root)

```bash
S=<scratchpad>
go run ./cmd/leafwiki --help > $S/help.txt 2>&1
python3 .claude/skills/docs-review/scripts/check_flags.py $S/help.txt README.md docs/*.md docs/*/*.md
python3 .claude/skills/docs-review/scripts/check_links.py README.md CONTRIBUTING.md docs/*.md docs/*/*.md
```

- `check_flags.py`: flags/env vars in the binary but missing from the docs, documented names
  that no longer exist, table defaults that differ from `--help`.
- `check_links.py`: relative links and `#anchors` (GitHub slug rules; inline code is skipped).
- `.env.example` vs code: every `LEAFWIKI_*` from `--help` should appear (commented is fine;
  `LEAFWIKI_ARCH`/`LEAFWIKI_VERSION` are installer-only, the deprecated
  `LEAFWIKI_HTTP_REMOTE_USER_LOGOUT_URL` may be omitted):
  ```bash
  grep -oE '\[\$LEAFWIKI_[A-Z_]+\]' $S/help.txt | tr -d '[]$' | sort -u > $S/code-env.txt
  grep -oE '^#? ?LEAFWIKI_[A-Z_]+' .env.example | sed 's/^# *//' | sort -u > $S/envex.txt
  comm -23 $S/code-env.txt $S/envex.txt
  ```
- New flag without a "Since" version: find it with
  `git log --reverse --format=%H -S'"<flag-name>"' -- cmd internal | head -1` and
  `git tag --contains <sha> --sort=v:refname | head -1`.

## 2. Manual checks

- **Language lists** must match `ui/leafwiki-ui/src/locales/*/`: README features + contributing
  line, `--default-language` row in `reference.md`, the table in `docs/development/i18n.md`
  (native names from each locale's `common.json` `selfName`).
- **UI labels** quoted in docs (bold names like **Full Backup**, **Git Content Backup**,
  **Refresh**, **Force Push**, **Sort Section Children**) must exist in
  `ui/leafwiki-ui/src/locales/en/*.json`.
- **Code locations** named in docs (e.g. `allowedLanguages` in `internal/usersettings/language.go`,
  `ui/leafwiki-ui/src/lib/dateLocale.ts`) still exist.
- **Behaviour claims** for whatever changed since the last tag (`git log v<prev>..HEAD`): read the
  code before trusting the doc. Example from the first run: git backup settings-mode only applies
  when `--git-backup` is not `true` (`buildBackupManager` in `cmd/leafwiki/main.go`).
- **Developer docs and ADRs** (`docs/development/`, `AGENTS.md`): file paths, function names and
  commands they mention still exist; an ADR whose decision was reversed is marked superseded.
- **Dates/offers** that go stale: time-limited offers and beta wording, "Since" columns, version-pinned commands
  (`git switch --detach vX.Y.Z` in `docs/install/binary.md` → latest tag).

## 3. Undocumented features — ask, don't write

Compare `ui/leafwiki-ui/src/features/*` and the backend domains in `internal/wiki/*` with the docs.
Known gaps at the first run (2026-10-09), unless documented since: Markdown syntax (`[[Title]]`
wikilinks, `==highlight==`, callouts, collapsible blocks, image sizing, PDF embed), tags and
properties frontmatter, `.leafwikiignore`, favorites/pins, 2FA setup, user settings
(language, date format, avatar, autosave), Markdown import, Prometheus metrics, API keys.

List the gaps and **ask the maintainer which to document** before writing new pages. New pages go into
the matching `docs/` folder and get a line in `docs/README.md` (and the README docs list if
they're user-facing).

## 4. Report

Short list per category: fixed automatically (factual drift), proposed (needs a decision),
undocumented features (question). Fix factual drift directly; anything that adds or removes
user-facing statements needs the maintainer's OK. Commit/push only after asking.
