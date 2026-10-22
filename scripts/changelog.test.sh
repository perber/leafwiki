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

expect_no_line() {
    local name="$1" output="$2" line="$3"
    if grep -qF -- "$line" <<< "$output"; then
        echo "FAIL $name: unexpected '$line'"
        FAILURES=$((FAILURES + 1))
    else
        echo "PASS $name: no '$line'"
    fi
}

# Users see what they notice; internal work collapses into one count line.
repo="$(new_repo filter \
    "feat(editor): new thing" \
    "fix(preview): visible bug" \
    "perf(search): faster" \
    "style: nicer blocks" \
    "fix(test): flaky test" \
    "fix(ci): broken job" \
    "docs(development): notes" \
    "refactor(tree): split files" \
    "test: more cases" \
    "chore: tidy" \
    "ci: new check" \
    "build: pin go" \
    "refactor(api)!: drop old route" \
    "Update README")"
out="$(changelog "$repo")"
expect_line filter "$out" "### ✨ Features"
expect_line filter "$out" "- feat(editor): new thing (Tester)"
expect_line filter "$out" "### 🐛 Bug Fixes"
expect_line filter "$out" "- fix(preview): visible bug (Tester)"
expect_line filter "$out" "### ⚡ Performance"
expect_line filter "$out" "- perf(search): faster (Tester)"
expect_line filter "$out" "### 💅 Style"
expect_line filter "$out" "- style: nicer blocks (Tester)"
expect_line filter "$out" "### 🔹 Other Changes"
expect_line filter "$out" "- refactor(api)!: drop old route (Tester)"
expect_line filter "$out" "- Update README (Tester)"
for hidden in "fix(test)" "fix(ci)" "docs(development)" "refactor(tree)" "test: more" "chore: tidy" "ci: new" "build: pin"; do
    expect_no_line filter "$out" "$hidden"
done
expect_line filter "$out" "Plus 8 internal changes (refactoring, tests, CI, build and developer docs) that don't change how LeafWiki behaves. Full list: [v1...v2](https://github.com/perber/leafwiki/compare/v1...v2)"

# No internal commits, no count line.
repo="$(new_repo no-internal "fix: only a fix")"
out="$(changelog "$repo")"
expect_no_line no-internal "$out" "internal change"

# expect_output <name> <output> <want>: the whole changelog, byte for byte.
expect_output() {
    local name="$1" output="$2" want="$3"
    if [[ "$output" == "$want" ]]; then
        echo "PASS $name: output matches"
    else
        echo "FAIL $name: output differs"
        diff <(printf '%s\n' "$want") <(printf '%s\n' "$output") | sed 's/^/    /'
        FAILURES=$((FAILURES + 1))
    fi
}

COUNT_LINE="Plus 1 internal change (refactoring, tests, CI, build and developer docs) that don't change how LeafWiki behaves. Full list: [v1...v2](https://github.com/perber/leafwiki/compare/v1...v2)"

# Without a Behavior-Change trailer the output is what it was before the
# trailer existed. A "Behavior-Change:" line followed by prose isn't a trailer.
repo="$(new_repo no-trailer "refactor: internal" "feat: a toggle

Behavior-Change: is only mentioned here,
so this paragraph is not a trailer block.")"
out="$(changelog "$repo")"
expect_output no-trailer "$out" "## 📝 Changelog for v2

### ✨ Features
- feat: a toggle (Tester)

$COUNT_LINE"

# Each trailer value is one bullet in a section above all others, newest
# commit first; a trailer on an internal commit still shows up.
repo="$(new_repo trailers "fix(search): rank titles higher

Behavior-Change: Search results are ordered differently; rebuild the index once." "refactor(storage): move attachments

Behavior-Change: Attachments move to data/assets on first start; back up first.
Behavior-Change: The --assets-dir flag is gone.")"
out="$(changelog "$repo")"
expect_output trailers "$out" "## 📝 Changelog for v2

### ⚠️ Behavior changes
- Attachments move to data/assets on first start; back up first.
- The --assets-dir flag is gone.
- Search results are ordered differently; rebuild the index once.

### 🐛 Bug Fixes
- fix(search): rank titles higher (Tester)

$COUNT_LINE"

# Values are printed literally: no expansion, no escapes, no option parsing.
repo="$(new_repo special-characters "chore: rename env var

Behavior-Change: Rename \`LEAFWIKI_X\` to \$LEAFWIKI_Y; \"quotes\", 'single', \\n and \$(whoami) stay as-is." "chore: drop flag

Behavior-Change: -n is no longer accepted.")"
out="$(changelog "$repo")"
expect_line special-characters "$out" "- Rename \`LEAFWIKI_X\` to \$LEAFWIKI_Y; \"quotes\", 'single', \\n and \$(whoami) stay as-is."
expect_line special-characters "$out" "- -n is no longer accepted."

if [[ "$FAILURES" -gt 0 ]]; then
    echo "$FAILURES failure(s)"
    exit 1
fi
echo "all changelog tests passed"
