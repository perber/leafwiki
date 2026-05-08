#!/usr/bin/env bash
# Run the reverse-proxy auth E2E test suite.
# Builds the LeafWiki image, starts the Docker Compose stack, runs Go tests,
# and tears everything down regardless of the outcome.
set -euo pipefail

cd "$(dirname "$0")"

PROXY_URL="${E2E_PROXY_URL:-http://localhost:8095}"

cleanup() {
  echo "🛑 Stopping proxy stack..."
  docker compose down --volumes --remove-orphans 2>/dev/null || true
}
trap cleanup EXIT

echo "🐳 Starting proxy E2E stack..."
docker compose up -d --build --wait

echo "✅ Stack ready. Running proxy auth tests..."
E2E_PROXY_URL="$PROXY_URL" go test -v -timeout 60s ./...

echo "✅ All proxy auth E2E tests passed."
