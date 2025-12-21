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

# Collect commits
COMMITS=$(git log "$PREVIOUS_TAG".."$CURRENT_TAG" --pretty=format:"%s (@%an)")

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