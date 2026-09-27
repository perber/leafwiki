#!/usr/bin/env bash

# -----------------------------------------------------------------------------
# changelog.sh
#
# Purpose:
#   Generates a categorized changelog in markdown format from git commit messages
#   between two tags.
#
# Expected commit message format:
#   Uses Conventional Commits prefixes for categorization:
#     feat:      New features
#     fix:       Bug fixes
#     docs:      Documentation changes
#     refactor:  Code refactoring
#     test:      Test-related changes
#     chore:     Maintenance tasks
#   Any commit not matching these prefixes is categorized as "Other Changes".
#
# Usage:
#   ./changelog.sh <previous_tag> <current_tag>
#
# Output:
#   Writes a markdown changelog to 'changelog.md' in the current directory.
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

# Collect commits
COMMITS=""
while IFS=$'\t' read -r hash subject; do
  [[ -z "$hash" ]] && continue
  author=$(resolve_author "$hash" "$subject")
  COMMITS+="$subject ($author)"$'\n'
done < <(git log "$PREVIOUS_TAG".."$CURRENT_TAG" --pretty=format:"%H%x09%s")
COMMITS="${COMMITS%$'\n'}"

# Categorize exclusively
FEATURES=""
FIXES=""
DOCS=""
REFACTOR=""
TESTS=""
CHORES=""
OTHERS=""
while IFS= read -r commit; do
  if [[ "$commit" =~ ^feat ]]; then
    FEATURES+="$commit"$'\n'
  elif [[ "$commit" =~ ^fix ]]; then
    FIXES+="$commit"$'\n'
  elif [[ "$commit" =~ ^docs ]]; then
    DOCS+="$commit"$'\n'
  elif [[ "$commit" =~ ^refactor ]]; then
    REFACTOR+="$commit"$'\n'
  elif [[ "$commit" =~ ^test ]]; then
    TESTS+="$commit"$'\n'
  elif [[ "$commit" =~ ^chore ]]; then
    CHORES+="$commit"$'\n'
  else
    OTHERS+="$commit"$'\n'
  fi
done <<< "$COMMITS"

# Build markdown file
OUTFILE="current_release_changelog.md"

{
  echo "## 📝 Changelog for $CURRENT_TAG"
  echo ""

  if [ -n "$FEATURES" ]; then
    echo "### ✨ Features"
    echo "$FEATURES" | sed '/^$/d; s/^/- /'
    echo ""
  fi
  if [ -n "$FIXES" ]; then
    echo "### 🐛 Bug Fixes"
    echo "$FIXES" | sed '/^$/d; s/^/- /'
    echo ""
  fi
  if [ -n "$DOCS" ]; then
    echo "### 🧾 Documentation"
    echo "$DOCS" | sed '/^$/d; s/^/- /'
    echo ""
  fi
  if [ -n "$REFACTOR" ]; then
    echo "### 🔧 Refactoring"
    echo "$REFACTOR" | sed '/^$/d; s/^/- /'
    echo ""
  fi
  if [ -n "$TESTS" ]; then
    echo "### 🧪 Tests"
    echo "$TESTS" | sed '/^$/d; s/^/- /'
    echo ""
  fi
  if [ -n "$CHORES" ]; then
    echo "### 🧰 Chores"
    echo "$CHORES" | sed '/^$/d; s/^/- /'
    echo ""
  fi
  if [ -n "$OTHERS" ]; then
    echo "### 🔹 Other Changes"
    echo "$OTHERS" | sed '/^$/d; s/^/- /'
    echo ""
  fi
} > "$OUTFILE"

echo "✅ Changelog written to $OUTFILE"