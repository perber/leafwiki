#!/bin/bash
# Tests changelog.sh against throwaway repositories. A stub `gh` that always
# fails keeps the PR-author lookup offline, so authors fall back to git names.
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/bin"
printf '#!/bin/sh\nexit 1\n' > "$WORK/bin/gh"
chmod +x "$WORK/bin/gh"

FAILURES=0

# new_repo <name> <subject>...: a repository with tag v1 on an initial commit,
# then one empty commit per subject (oldest first) and tag v2 on the last one.
new_repo() {
    local repo="$WORK/$1"
    shift
    git init -q "$repo"
    git -C "$repo" -c user.name=Tester -c user.email=t@example.com commit -q --allow-empty -m "chore: initial"
    git -C "$repo" tag v1
    local subject
    for subject in "$@"; do
        git -C "$repo" -c user.name=Tester -c user.email=t@example.com commit -q --allow-empty -m "$subject"
    done
    git -C "$repo" tag v2
    echo "$repo"
}

# changelog <repo>: runs changelog.sh v1 v2 in <repo> and prints the result.
changelog() {
    (cd "$1" && PATH="$WORK/bin:$PATH" "$ROOT/scripts/changelog.sh" v1 v2 >/dev/null) &&
        cat "$1/current_release_changelog.md"
}

expect_line() {
    local name="$1" output="$2" line="$3"
    if grep -qxF -- "$line" <<< "$output"; then
        echo "PASS $name: has '$line'"
    else
        echo "FAIL $name: missing '$line'"
        FAILURES=$((FAILURES + 1))
    fi
}

# Regression: git log --pretty=format: ends without a newline, and the read
# loop dropped the last entry, which is the oldest commit of the release.
repo="$(new_repo oldest "fix: oldest change" "fix: newest change")"
out="$(changelog "$repo")"
expect_line oldest-commit "$out" "- fix: oldest change (Tester)"
expect_line oldest-commit "$out" "- fix: newest change (Tester)"

if [[ "$FAILURES" -gt 0 ]]; then
    echo "$FAILURES failure(s)"
    exit 1
fi
echo "all changelog tests passed"
