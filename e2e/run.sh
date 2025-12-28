#!/usr/bin/env bash

set -euo pipefail

current_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

start_docker() {
  echo "🟢 Starting Docker container..."
  docker build -t wiki-e2e-tests $current_dir/../

  # Remove old container beforehand, if present
  if docker ps -a --format '{{.Names}}' | grep -q '^wiki-e2e-tests$'; then
    echo "⚠️ Removing existing container..."
    docker rm -f wiki-e2e-tests >/dev/null 2>&1 || true
  fi

  docker run -d \
    -p 8085:8080 \
    --name wiki-e2e-tests \
    -v e2e-tests-data:/app/data \
    wiki-e2e-tests \
    --allow-insecure=true \
    --jwt-secret=e2e-tests-secret \
    --admin-password=admin

  echo "✅ Container started on http://localhost:8085"
}

stop_docker() {
  echo "🛑 Stopping Docker container..."
  docker stop wiki-e2e-tests >/dev/null 2>&1 || true
  docker rm wiki-e2e-tests >/dev/null 2>&1 || true
}

run_playwright_tests() {
  echo "Running Playwright tests..."
  cd "$current_dir" && \
  E2E_BASE_URL="http://localhost:8085" \
  E2E_ADMIN_USER="admin" \
  E2E_ADMIN_PASSWORD="admin" \
  npx playwright test
}

if nc -z localhost 8085 >/dev/null 2>&1; then
  echo "⚠️ Port 8085 already in use – restarting container..."
  stop_docker
fi
start_docker
trap stop_docker EXIT

# wait until the wiki is reachable
# but max for 2 minutes
max_attempts=60
attempt=0
until curl -s http://localhost:8085 >/dev/null; do
  printf '.'
  sleep 2
  attempt=$((attempt + 1))
  if [ "$attempt" -ge "$max_attempts" ]; then
    echo "❌ LeafWiki is not reachable after 2 minutes."
    exit 1
  fi
done
echo "✅ LeafWiki is reachable."

run_playwright_tests
