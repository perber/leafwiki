#!/bin/bash
# Fails when a non-merge commit in the given range doesn't have a Conventional
# Commit subject (CONTRIBUTING.md#commit-style). Rebase merges keep these
# commits on main, and scripts/changelog.sh groups the release notes by type.
#
# Usage: scripts/check-commit-messages.sh <base>..<head>
set -euo pipefail

if [[ $# -ne 1 ]]; then
    echo "Usage: $0 <base>..<head>" >&2
    exit 2
fi
RANGE="$1"

TYPES='build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test'
CONVENTIONAL="^(${TYPES})(\([A-Za-z0-9][A-Za-z0-9._/-]*\))?!?: [^ ]"
# GitHub's "Revert" button produces `Revert "<original subject>"`.
GITHUB_REVERT='^Revert ".+"$'

# Workflow commands treat %, CR and LF specially in the message.
escape() {
    local s="${1//'%'/'%25'}"
    s="${s//$'\r'/'%0D'}"
    printf '%s' "${s//$'\n'/'%0A'}"
}

log="$(git log --no-merges --format='%h%x09%s' "$RANGE")"

failed=0
while IFS=$'\t' read -r sha subject; do
    [[ -z "$sha" ]] && continue
    if [[ "$subject" =~ $CONVENTIONAL || "$subject" =~ $GITHUB_REVERT ]]; then
        continue
    fi
    echo "::error title=Commit ${sha}::Not a Conventional Commit subject: $(escape "$subject")"
    failed=1
done <<< "$log"

if [[ "$failed" -ne 0 ]]; then
    echo "Use \"type(scope): description\" with type one of ${TYPES//|/, };" \
        "fixup!/squash!/WIP commits must be squashed first. See CONTRIBUTING.md#commit-style."
    exit 1
fi
echo "All commit subjects in ${RANGE} are Conventional Commits."
