#!/usr/bin/env bash

# -----------------------------------------------------------------------------
# changelog.sh
#
# Purpose:
#   Generates the release changelog in markdown from git commit messages
#   between two tags. It lists what users notice and only counts the rest.
#
# Expected commit message format:
#   Conventional Commits. Listed by section:
#     feat:      Features
#     fix:       Bug Fixes (except fix(test) and fix(ci))
#     perf:      Performance
#     style:     Style
#   Breaking changes (type!:) and subjects without a type go to "Other Changes".
#   Everything else (docs, refactor, test, chore, ci, build, fix(test),
#   fix(ci)) is internal: one line with the count and a compare link.
#
# Usage:
#   ./changelog.sh <previous_tag> <current_tag>
#   GITHUB_REPOSITORY (owner/repo) sets the compare link; default perber/leafwiki.
#
# Output:
#   Writes a markdown changelog to 'current_release_changelog.md' in the
#   current directory.
# -----------------------------------------------------------------------------
set -euo pipefail

# === Configuration ===
PREVIOUS_TAG="${1:-}"
CURRENT_TAG="${2:-}"

if [[ -z "$PREVIOUS_TAG" || -z "$CURRENT_TAG" ]]; then
  echo "Usage: $0 <previous_tag> <current_tag>"
  exit 1
fi

echo "🔍 Generating changelog from $PREVIOUS_TAG → $CURRENT_TAG"

# Validate tags exist
if ! git rev-parse --verify "$PREVIOUS_TAG" >/dev/null 2>&1; then
  echo "❌ Previous tag '$PREVIOUS_TAG' does not exist."
  exit 1
fi
if ! git rev-parse --verify "$CURRENT_TAG" >/dev/null 2>&1; then
  echo "❌ Current tag '$CURRENT_TAG' does not exist."
  exit 1
fi

HAVE_GH=0
if command -v gh >/dev/null 2>&1; then
  HAVE_GH=1
else
  echo "⚠️  'gh' not found, falling back to git author names instead of GitHub handles."
fi

declare -A PR_AUTHOR_CACHE

# Resolves a commit subject to "@<github-handle>" via its PR number when
# possible (GitHub squash-merges append "(#1234)" to the subject); falls back
# to the git commit author's display name otherwise.
resolve_author() {
  local commit_hash="$1"
  local subject="$2"
  local pr_number=""

  if [[ "$subject" =~ \(#([0-9]+)\)[[:space:]]*$ ]]; then
    pr_number="${BASH_REMATCH[1]}"
  fi

  if [[ -n "$pr_number" && "$HAVE_GH" -eq 1 ]]; then
    if [[ -z "${PR_AUTHOR_CACHE[$pr_number]+set}" ]]; then
      PR_AUTHOR_CACHE[$pr_number]=$(gh pr view "$pr_number" --json author -q '.author.login' 2>/dev/null || true)
    fi
    if [[ -n "${PR_AUTHOR_CACHE[$pr_number]}" ]]; then
      echo "@${PR_AUTHOR_CACHE[$pr_number]}"
      return
    fi
  fi

  git log -1 --format="%an" "$commit_hash"
}

# Collect commits. tformat ends every entry with a newline; with format: the
# last entry (the oldest commit) had none, and `read` dropped it.
COMMITS=""
while IFS=$'\t' read -r hash subject; do
  [[ -z "$hash" ]] && continue
  author=$(resolve_author "$hash" "$subject")
  COMMITS+="$subject ($author)"$'\n'
done < <(git log "$PREVIOUS_TAG".."$CURRENT_TAG" --pretty=tformat:"%H%x09%s")
COMMITS="${COMMITS%$'\n'}"

# Categorize exclusively: user-facing types get a section, internal work is
# only counted.
TYPE_RE='^([a-z]+)(\(([^)]*)\))?(!)?:'
FEATURES=""
FIXES=""
PERF=""
STYLE=""
OTHERS=""
INTERNAL=0
while IFS= read -r commit; do
  [[ -z "$commit" ]] && continue
  if [[ ! "$commit" =~ $TYPE_RE ]]; then
    OTHERS+="$commit"$'\n'
    continue
  fi
  type="${BASH_REMATCH[1]}"
  scope="${BASH_REMATCH[3]}"
  breaking="${BASH_REMATCH[4]}"
  case "$type" in
    feat) FEATURES+="$commit"$'\n' ;;
    fix)
      if [[ -z "$breaking" && ( "$scope" == "test" || "$scope" == "ci" ) ]]; then
        INTERNAL=$((INTERNAL + 1))
      else
        FIXES+="$commit"$'\n'
      fi
      ;;
    perf) PERF+="$commit"$'\n' ;;
    style) STYLE+="$commit"$'\n' ;;
    *)
      if [[ -n "$breaking" ]]; then
        OTHERS+="$commit"$'\n'
      else
        INTERNAL=$((INTERNAL + 1))
      fi
      ;;
  esac
done <<< "$COMMITS"

REPOSITORY="${GITHUB_REPOSITORY:-perber/leafwiki}"

# print_section <heading> <entries>
print_section() {
  [[ -z "$2" ]] && return
  echo "### $1"
  echo "$2" | sed '/^$/d; s/^/- /'
  echo ""
}

# Build markdown file
OUTFILE="current_release_changelog.md"

{
  echo "## 📝 Changelog for $CURRENT_TAG"
  echo ""
  print_section "✨ Features" "$FEATURES"
  print_section "🐛 Bug Fixes" "$FIXES"
  print_section "⚡ Performance" "$PERF"
  print_section "💅 Style" "$STYLE"
  print_section "🔹 Other Changes" "$OTHERS"
  if [[ "$INTERNAL" -gt 0 ]]; then
    if [[ "$INTERNAL" -eq 1 ]]; then noun="change"; else noun="changes"; fi
    echo "Plus $INTERNAL internal $noun (refactoring, tests, CI, build and developer docs) that don't change how LeafWiki behaves. Full list: [$PREVIOUS_TAG...$CURRENT_TAG](https://github.com/$REPOSITORY/compare/$PREVIOUS_TAG...$CURRENT_TAG)"
  fi
} > "$OUTFILE"

echo "✅ Changelog written to $OUTFILE"
