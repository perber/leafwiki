#!/usr/bin/env bash
set -euo pipefail

# Single source of truth for the LeafWiki build version, used by both the
# Go build (main.Version via -ldflags) and the frontend build
# (__APP_VERSION__ via vite.config.ts) so they can never drift out of sync.
if [ -n "${APP_VERSION:-}" ]; then
  echo "$APP_VERSION"
  exit 0
fi

git describe --tags --abbrev=0 2>/dev/null || echo "v0.1.0"
