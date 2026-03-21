#!/usr/bin/env bash

set -euo pipefail

current_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$current_dir/.." && pwd)"
app_url="${E2E_BASE_URL:-http://localhost:8085}"
app_port="${E2E_PORT:-8085}"
run_mode="${E2E_RUN_MODE:-docker}"
server_pid=""
server_log="$current_dir/local-server.log"
local_data_dir=""

build_frontend_for_local_e2e() {
  if [ "${E2E_SKIP_UI_BUILD:-0}" = "1" ]; then
    echo "⚡ Skipping UI build for local E2E run..."
  else
    echo "🔨 Building frontend for local E2E run..."
    (
      cd "$repo_root/ui/leafwiki-ui"
      npm run build
    )
  fi

  if [ ! -f "$repo_root/ui/leafwiki-ui/dist/index.html" ]; then
    echo "❌ Frontend build output is missing at ui/leafwiki-ui/dist/index.html"
    exit 1
  fi

  rm -rf "$repo_root/internal/http/dist"
  mkdir -p "$repo_root/internal/http/dist"
  cp -R "$repo_root/ui/leafwiki-ui/dist/." "$repo_root/internal/http/dist/"
  touch "$repo_root/internal/http/dist/.gitkeep"
}

start_docker() {
  echo "🟢 Starting Docker container..."
  docker build -t wiki-e2e-tests "$repo_root"

  if docker ps -a --format '{{.Names}}' | grep -q '^wiki-e2e-tests$'; then
    echo "⚠️ Removing existing container..."
    docker rm -f wiki-e2e-tests >/dev/null 2>&1 || true
  fi

  docker run -d \
    -p "$app_port:8080" \
    --name wiki-e2e-tests \
    -v e2e-tests-data:/app/data \
    wiki-e2e-tests \
    --allow-insecure=true \
    --jwt-secret=e2e-tests-secret \
    --admin-password=admin

  echo "✅ Container started on $app_url"
}

stop_docker() {
  echo "🛑 Stopping Docker container..."
  docker stop wiki-e2e-tests >/dev/null 2>&1 || true
  docker rm wiki-e2e-tests >/dev/null 2>&1 || true
  docker rmi wiki-e2e-tests >/dev/null 2>&1 || true
}

start_local() {
  echo "🟢 Starting local LeafWiki process..."
  build_frontend_for_local_e2e

  local_data_dir="$(mktemp -d /tmp/leafwiki-e2e-data.XXXXXX)"
  : > "$server_log"

  (
    cd "$repo_root"
    go run \
      -ldflags="-X github.com/perber/wiki/internal/http.EmbedFrontend=true -X github.com/perber/wiki/internal/http.Environment=production" \
      ./cmd/leafwiki/main.go \
      --host 127.0.0.1 \
      --port "$app_port" \
      --data-dir "$local_data_dir" \
      --allow-insecure=true \
      --jwt-secret=e2e-tests-secret \
      --admin-password=admin
  ) >"$server_log" 2>&1 &

  server_pid=$!
  echo "✅ Local process started on $app_url (pid $server_pid)"
}

stop_local() {
  echo "🛑 Stopping local LeafWiki process..."
  if [ -n "$server_pid" ] && kill -0 "$server_pid" >/dev/null 2>&1; then
    kill "$server_pid" >/dev/null 2>&1 || true
    wait "$server_pid" >/dev/null 2>&1 || true
  fi

  if [ -n "$local_data_dir" ] && [ -d "$local_data_dir" ]; then
    rm -rf "$local_data_dir"
  fi
}

stop_runner() {
  if [ "$run_mode" = "docker" ]; then
    stop_docker
  else
    stop_local
  fi
}

run_playwright_tests() {
  echo "Running Playwright tests..."
  (
    cd "$current_dir"
    E2E_BASE_URL="$app_url" \
    E2E_ADMIN_USER="${E2E_ADMIN_USER:-admin}" \
    E2E_ADMIN_PASSWORD="${E2E_ADMIN_PASSWORD:-admin}" \
    npx playwright test "$@"
  )
}

wait_until_reachable() {
  local max_attempts=60
  local attempt=0

  until curl -s "$app_url" >/dev/null; do
    printf '.'
    sleep 2
    attempt=$((attempt + 1))
    if [ "$attempt" -ge "$max_attempts" ]; then
      echo
      echo "❌ LeafWiki is not reachable after 2 minutes."
      if [ "$run_mode" = "local" ] && [ -f "$server_log" ]; then
        echo "--- local server log ---"
        tail -n 200 "$server_log" || true
      fi
      exit 1
    fi
  done

  echo
  echo "✅ LeafWiki is reachable."
}

if nc -z localhost "$app_port" >/dev/null 2>&1; then
  if docker ps -a --format '{{.Names}}' | grep -q '^wiki-e2e-tests$'; then
    echo "⚠️ Port $app_port already in use by an existing E2E container – restarting it..."
    stop_docker
  else
    echo "❌ Port $app_port is already in use. Stop the existing process or choose another E2E_PORT."
    exit 1
  fi
fi

if [ "$run_mode" = "docker" ]; then
  start_docker
else
  start_local
fi
trap stop_runner EXIT

wait_until_reachable
run_playwright_tests "$@"
