#!/usr/bin/env bash
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

# Categorize
FEATURES=$(echo "$COMMITS" | grep -Ei '^feat|feature' || true)
FIXES=$(echo "$COMMITS" | grep -Ei '^fix|bug' || true)
DOCS=$(echo "$COMMITS" | grep -Ei '^docs' || true)
REFACTOR=$(echo "$COMMITS" | grep -Ei '^refactor' || true)
TESTS=$(echo "$COMMITS" | grep -Ei '^test' || true)
CHORES=$(echo "$COMMITS" | grep -Ei '^chore' || true)
OTHERS=$(echo "$COMMITS" | grep -Evi '^(feat|fix|bug|docs|refactor|test|chore)' || true)

# Build markdown file
OUTFILE="changelog.md"

{
  echo "## 📝 Changelog for $CURRENT_TAG"
  echo ""

  if [ -n "$FEATURES" ]; then
    echo "### ✨ Features"
    echo "$FEATURES" | sed 's/^/- /'
    echo ""
  fi
  if [ -n "$FIXES" ]; then
    echo "### 🐛 Bug Fixes"
    echo "$FIXES" | sed 's/^/- /'
    echo ""
  fi
  if [ -n "$DOCS" ]; then
    echo "### 🧾 Documentation"
    echo "$DOCS" | sed 's/^/- /'
    echo ""
  fi
  if [ -n "$REFACTOR" ]; then
    echo "### 🔧 Refactoring"
    echo "$REFACTOR" | sed 's/^/- /'
    echo ""
  fi
  if [ -n "$TESTS" ]; then
    echo "### 🧪 Tests"
    echo "$TESTS" | sed 's/^/- /'
    echo ""
  fi
  if [ -n "$CHORES" ]; then
    echo "### 🧰 Chores"
    echo "$CHORES" | sed 's/^/- /'
    echo ""
  fi
  if [ -n "$OTHERS" ]; then
    echo "### 🔹 Other Changes"
    echo "$OTHERS" | sed 's/^/- /'
    echo ""
  fi
} > "$OUTFILE"

echo "✅ Changelog written to $OUTFILE"