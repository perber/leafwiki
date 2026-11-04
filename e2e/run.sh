#!/usr/bin/env bash

set -euo pipefail

current_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$current_dir/.." && pwd)"
app_port="${E2E_PORT:-8085}"
app_url="${E2E_BASE_URL:-http://localhost:${app_port}}"
run_mode="${E2E_RUN_MODE:-docker}"
# Local mode starts one server per Playwright worker (own port and data dir),
# so parallel workers never see each other's pages, users or settings.
# Docker mode keeps one container and one worker.
# ports is comma-separated; playwright.config.ts maps worker N to the Nth one.
ports="$app_port"
if [ "$run_mode" = "local" ]; then
  workers="${E2E_PLAYWRIGHT_WORKERS:-$(( $(getconf _NPROCESSORS_ONLN) / 2 ))}"
  [ "$workers" -ge 1 ] || workers=1
  for ((i = 1; i < workers; i++)); do ports="$ports,$((app_port + i))"; done
else
  workers="${E2E_PLAYWRIGHT_WORKERS:-1}"
fi
local_run_dir=""
docker_data_volume=""

print_runner_diagnostics() {
  echo "--- E2E runtime diagnostics ---"

  if [ "$run_mode" = "docker" ]; then
    if docker ps -a --format '{{.Names}}' | grep -q '^wiki-e2e-tests$'; then
      echo "--- docker logs (last 200 lines) ---"
      docker logs --tail 200 wiki-e2e-tests 2>&1 || true
    else
      echo "No Docker container logs available."
    fi
    return
  fi

  local log
  for log in "${local_run_dir:-/nonexistent}"/server-*.log; do
    [ -f "$log" ] || { echo "No local server log available."; continue; }
    echo "--- local server log $log (last 200 lines) ---"
    tail -n 200 "$log" || true
  done
}

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
  docker build \
    --build-arg DISABLE_REFRESH_TOKEN_RATE_LIMIT=true \
    -t wiki-e2e-tests \
    "$repo_root"

  if docker ps -a --format '{{.Names}}' | grep -q '^wiki-e2e-tests$'; then
    echo "⚠️ Removing existing container..."
    docker rm -f wiki-e2e-tests >/dev/null 2>&1 || true
  fi

  docker_data_volume="wiki-e2e-tests-data-${RANDOM}${RANDOM}"
  docker volume create "$docker_data_volume" >/dev/null

  docker run -d \
    -p "$app_port:8080" \
    --name wiki-e2e-tests \
    -v "$docker_data_volume":/app/data \
    wiki-e2e-tests \
    --allow-insecure=true \
    --enable-revision=true \
    --enable-link-refactor=true \
    --revision-coalesce-window=0 \
    --jwt-secret=e2e-tests-secret \
    --totp-encryption-key=e2e-tests-totp-encryption-key-32 \
    --admin-password=admine2epassword

  echo "✅ Container started on $app_url"
}

stop_docker() {
  echo "🛑 Stopping Docker container..."
  docker stop wiki-e2e-tests >/dev/null 2>&1 || true
  docker rm wiki-e2e-tests >/dev/null 2>&1 || true
  docker rmi wiki-e2e-tests >/dev/null 2>&1 || true
  if [ -n "$docker_data_volume" ]; then
    docker volume rm "$docker_data_volume" >/dev/null 2>&1 || true
  fi
}

start_local() {
  echo "🟢 Starting local LeafWiki process..."
  build_frontend_for_local_e2e

  local_run_dir="$(mktemp -d /tmp/leafwiki-e2e.XXXXXX)"
  local server_bin="$local_run_dir/leafwiki"

  echo "🔨 Building leafwiki binary for local E2E run..."
  (
    cd "$repo_root"
    go build -tags nomsgpack \
      -ldflags="-X github.com/perber/wiki/internal/http.EmbedFrontend=true -X github.com/perber/wiki/internal/http.Environment=production -X github.com/perber/wiki/internal/wiki/auth.DisableRefreshTokenRateLimit=true" \
      -o "$server_bin" \
      ./cmd/leafwiki
  )

  # Run the binary directly in the background, not via `go run`: `$!` would
  # be the `go run` wrapper and `stop_local`'s `kill` would orphan the server.
  local i port
  for ((i = 0; i < workers; i++)); do
    port=$((app_port + i))
    mkdir -p "$local_run_dir/data-$i"
    (
      cd "$repo_root"
      exec "$server_bin" \
        --host 127.0.0.1 \
        --port "$port" \
        --data-dir "$local_run_dir/data-$i" \
        --allow-insecure=true \
        --enable-revision=true \
        --enable-link-refactor=true \
        --revision-coalesce-window=0 \
        --jwt-secret=e2e-tests-secret \
        --totp-encryption-key=e2e-tests-totp-encryption-key-32 \
        --admin-password=admine2epassword
    ) >"$local_run_dir/server-$i.log" 2>&1 &
    echo $! >>"$local_run_dir/pids"
    echo "✅ Local process started on http://localhost:$port (pid $!)"
  done
}

stop_local() {
  echo "🛑 Stopping local LeafWiki processes..."
  if [ -n "$local_run_dir" ] && [ -f "$local_run_dir/pids" ]; then
    local pid
    while read -r pid; do
      kill "$pid" >/dev/null 2>&1 || true
      wait "$pid" >/dev/null 2>&1 || true
    done <"$local_run_dir/pids"
  fi
  if [ -n "$local_run_dir" ] && [ -d "$local_run_dir" ]; then
    rm -rf "$local_run_dir"
  fi
}

stop_runner() {
  if [ "$run_mode" = "docker" ]; then
    stop_docker
  else
    stop_local
  fi
}

cleanup_runner() {
  local exit_code=$?

  if [ "$exit_code" -ne 0 ]; then
    print_runner_diagnostics
  fi

  stop_runner
  exit "$exit_code"
}

run_playwright_tests() {
  echo "Running Playwright tests..."
  (
    cd "$current_dir"
    local reporter="${E2E_PLAYWRIGHT_REPORTER:-line}"

    if command -v stdbuf >/dev/null 2>&1; then
      E2E_BASE_URL="$app_url" \
      E2E_PORTS="$([ "$run_mode" = "local" ] && echo "$ports")" \
      E2E_ADMIN_USER="${E2E_ADMIN_USER:-admin}" \
      E2E_ADMIN_PASSWORD="${E2E_ADMIN_PASSWORD:-admine2epassword}" \
      PLAYWRIGHT_FORCE_TTY=1 \
      stdbuf -oL -eL npx playwright test --workers="$workers" --reporter="$reporter" "$@"
    else
      E2E_BASE_URL="$app_url" \
      E2E_PORTS="$([ "$run_mode" = "local" ] && echo "$ports")" \
      E2E_ADMIN_USER="${E2E_ADMIN_USER:-admin}" \
      E2E_ADMIN_PASSWORD="${E2E_ADMIN_PASSWORD:-admine2epassword}" \
      PLAYWRIGHT_FORCE_TTY=1 \
      npx playwright test --workers="$workers" --reporter="$reporter" "$@"
    fi
  )
}

wait_until_reachable() {
  local url="$1"
  local max_attempts=60
  local attempt=0

  until curl -s "$url" >/dev/null; do
    printf '.'
    sleep 2
    attempt=$((attempt + 1))
    if [ "$attempt" -ge "$max_attempts" ]; then
      echo
      echo "❌ LeafWiki is not reachable at $url after 2 minutes."
      exit 1
    fi
  done

  echo
  echo "✅ LeafWiki is reachable at $url."
}

for port in ${ports//,/ }; do
  if nc -z localhost "$port" >/dev/null 2>&1; then
    if docker ps -a --format '{{.Names}}' | grep -q '^wiki-e2e-tests$'; then
      echo "⚠️ Port $port already in use by an existing E2E container – restarting it..."
      stop_docker
    else
      echo "❌ Port $port is already in use. Stop the existing process or choose another E2E_PORT."
      exit 1
    fi
  fi
done

if [ "$run_mode" = "docker" ]; then
  start_docker
else
  start_local
fi
trap cleanup_runner EXIT

if [ "$run_mode" = "docker" ]; then
  wait_until_reachable "$app_url"
else
  for port in ${ports//,/ }; do wait_until_reachable "http://localhost:$port"; done
fi
run_playwright_tests "$@"
