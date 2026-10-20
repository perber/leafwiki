#!/bin/bash
# Fails when tracked files reference things outside the public repository:
# a maintainer's private notes or absolute paths on someone's machine.
# Comments and docs reference GitHub issues or ADRs only
# (docs/development/rules.md, rule 9).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Generic tokens only; matched case-insensitively.
PRIVATE_NOTES='pleaf|ai-gen-infos'
# Absolute home directories (Linux, macOS): /home/<name> or /Users/<name>, in
# any case for the name, with or without a trailing slash. Windows paths are
# left out: tests use them on purpose for path handling.
HOME_PATHS='/(home|Users)/[A-Za-z_][A-Za-z0-9._-]*'

# Existing hits that are deliberate. This list may only shrink.
ALLOWED=(
    # Test data: a slug taken from a real page, harmless as a string.
    ':!ui/leafwiki-ui/src/features/preview/MarkdownPreview.test.tsx'
    ':!ui/leafwiki-ui/src/lib/preprocessWikilinks.test.ts'
)

EXCLUDED=(
    ':(exclude)scripts/check-private-references.sh'
    # In glob magic a leading **/ also matches zero directories, so these
    # cover top-level testdata/ and fixtures/ as well.
    ':(exclude,glob)**/testdata/**'
    ':(exclude,glob)**/fixtures/**'
    ':(exclude,glob)**/package-lock.json'
)

# git grep exits 0 on a match, 1 on none and >1 on an error; an error must
# not pass as "nothing found".
search() {
    local status=0
    git grep -n -I -E "$@" -- . "${EXCLUDED[@]}" "${ALLOWED[@]}" || status=$?
    if [[ "$status" -gt 1 ]]; then
        echo "::error::git grep failed (exit $status); is $ROOT a git checkout?" >&2
        exit 2
    fi
}

hits="$(
    search -i -e "$PRIVATE_NOTES"
    search -e "$HOME_PATHS"
)"

if [[ -n "$hits" ]]; then
    while IFS= read -r hit; do
        file="${hit%%:*}"
        rest="${hit#*:}"
        line="${rest%%:*}"
        echo "::error file=${file},line=${line}::Private reference: ${rest#*:}"
    done <<< "$hits"
    echo "Reference GitHub issues or ADRs instead (docs/development/rules.md, rule 9)."
    exit 1
fi
echo "No private references found."
