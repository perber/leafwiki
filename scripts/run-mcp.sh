#!/usr/bin/env bash
set -euo pipefail

leafwiki_bin="${LEAFWIKI_RUN_MCP_LEAFWIKI_BIN:-${LEAFWIKI_BIN:-leafwiki}}"
mcp_stdio_bin="${LEAFWIKI_RUN_MCP_STDIO_BIN:-${LEAFWIKI_MCP_STDIO_BIN:-leafwiki-mcp-stdio}}"
scheme="${LEAFWIKI_RUN_MCP_SCHEME:-http}"
host="${LEAFWIKI_RUN_MCP_HOST:-${LEAFWIKI_HOST:-127.0.0.1}}"
port="${LEAFWIKI_RUN_MCP_PORT:-${LEAFWIKI_PORT:-8080}}"
base_path="${LEAFWIKI_RUN_MCP_BASE_PATH:-${LEAFWIKI_BASE_PATH:-}}"
data_dir="${LEAFWIKI_RUN_MCP_DATA_DIR:-${LEAFWIKI_DATA_DIR:-./data}}"
root_dir="${LEAFWIKI_RUN_MCP_ROOT_DIR:-${LEAFWIKI_ROOT_DIR:-./wiki}}"
jwt_secret="${LEAFWIKI_RUN_MCP_JWT_SECRET:-${LEAFWIKI_JWT_SECRET:-p4lyOlQU643BRUc2HBiCrr55L6ygh4pJlVQ8z5LEnfT}}"
admin_password="${LEAFWIKI_RUN_MCP_ADMIN_PASSWORD:-${LEAFWIKI_ADMIN_PASSWORD:-admin}}"
allow_insecure="${LEAFWIKI_RUN_MCP_ALLOW_INSECURE:-${LEAFWIKI_ALLOW_INSECURE:-1}}"
disable_auth="${LEAFWIKI_RUN_MCP_DISABLE_AUTH:-${LEAFWIKI_DISABLE_AUTH:-0}}"
disable_request_log="${LEAFWIKI_RUN_MCP_DISABLE_REQUEST_LOG:-${LEAFWIKI_DISABLE_REQUEST_LOG:-1}}"
endpoint="${LEAFWIKI_RUN_MCP_ENDPOINT:-${LEAFWIKI_MCP_ENDPOINT:-}}"
health_url="${LEAFWIKI_RUN_MCP_HEALTH_URL:-}"
api_key="${LEAFWIKI_RUN_MCP_API_KEY:-${LEAFWIKI_MCP_API_KEY:-}}"
request_timeout="${LEAFWIKI_RUN_MCP_REQUEST_TIMEOUT:-${LEAFWIKI_MCP_STDIO_REQUEST_TIMEOUT:-}}"
shutdown_timeout="${LEAFWIKI_RUN_MCP_SHUTDOWN_TIMEOUT:-${LEAFWIKI_MCP_STDIO_SHUTDOWN_TIMEOUT:-}}"
max_frame_size="${LEAFWIKI_RUN_MCP_MAX_FRAME_SIZE:-${LEAFWIKI_MCP_STDIO_MAX_FRAME_SIZE:-}}"
ready_timeout="${LEAFWIKI_RUN_MCP_READY_TIMEOUT:-30}"
server_log="${LEAFWIKI_RUN_MCP_SERVER_LOG:-${TMPDIR:-/tmp}/leafwiki-run-mcp.$$.log}"
dry_run=0

server_extra_args=()
stdio_extra_args=()
server_pid=""
stdio_pid=""

usage() {
  cat <<EOF
Usage: scripts/run-mcp.sh [options]

Starts a local LeafWiki server with MCP enabled, then runs leafwiki-mcp-stdio
against that server. This wrapper is intended to be configured as the command
for MCP clients that only support spawning a STDIO MCP server process.

Options:
  --leafwiki-bin <path>     LeafWiki executable (default: leafwiki)
  --mcp-stdio-bin <path>    leafwiki-mcp-stdio executable (default: leafwiki-mcp-stdio)
  --scheme <scheme>         Endpoint scheme for computed URLs (default: http)
  --host <host>             LeafWiki bind host (default: 127.0.0.1)
  --port <port>             LeafWiki port (default: 8080)
  --base-path <path>        LeafWiki base path, if any
  --data-dir <path>         LeafWiki data directory (default: ./data)
  --root-dir <path>         LeafWiki root markdown directory (default: ./wiki)
  --jwt-secret <secret>     LeafWiki JWT secret (default: local development secret)
  --admin-password <pass>   LeafWiki initial admin password (default: admin)
  --disable-auth            Start LeafWiki with --disable-auth instead of JWT/admin auth
  --allow-insecure          Pass --allow-insecure to LeafWiki (default)
  --no-allow-insecure       Do not pass --allow-insecure
  --request-log             Keep LeafWiki request logs enabled
  --disable-request-log     Pass --disable-request-log to LeafWiki (default)
  --endpoint <url>          Upstream MCP URL for leafwiki-mcp-stdio (default: computed /mcp)
  --health-url <url>        URL polled before starting leafwiki-mcp-stdio (default: computed /api/health)
  --api-key <key>           MCP API key passed to leafwiki-mcp-stdio
  --request-timeout <dur>   leafwiki-mcp-stdio request timeout
  --shutdown-timeout <dur>  leafwiki-mcp-stdio shutdown timeout
  --max-frame-size <size>   leafwiki-mcp-stdio max frame size
  --ready-timeout <sec>     Seconds to wait for LeafWiki readiness (default: 30)
  --server-log <path>       LeafWiki server stdout/stderr log path
  --server-arg <arg>        Extra argument passed to leafwiki; repeatable
  --stdio-arg <arg>         Extra argument passed to leafwiki-mcp-stdio; repeatable
  --dry-run                 Print the planned commands without starting anything
  -h, --help                Show this help

Environment overrides use LEAFWIKI_RUN_MCP_* names matching the option names,
for example LEAFWIKI_RUN_MCP_PORT, LEAFWIKI_RUN_MCP_ROOT_DIR,
LEAFWIKI_RUN_MCP_ENDPOINT, and LEAFWIKI_RUN_MCP_API_KEY. Existing LeafWiki
environment variables such as LEAFWIKI_HOST, LEAFWIKI_PORT, LEAFWIKI_ROOT_DIR,
LEAFWIKI_JWT_SECRET, LEAFWIKI_ADMIN_PASSWORD, LEAFWIKI_MCP_ENDPOINT, and
LEAFWIKI_MCP_API_KEY are also honored as fallbacks.
EOF
}

log() {
  printf '%s\n' "$1" >&2
}

fail() {
  printf 'Error: %s\n' "$1" >&2
  exit 1
}

quote_command() {
  local arg
  for arg in "$@"; do
    printf '%q ' "$arg"
  done
}

print_command() {
  printf '+ ' >&2
  quote_command "$@" >&2
  printf '\n' >&2
}

truthy() {
  case "${1:-}" in
    1|true|TRUE|yes|YES|on|ON)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

require_executable() {
  local executable="$1"
  if [[ "$executable" == */* ]]; then
    [[ -x "$executable" ]] || fail "executable not found or not executable: $executable"
  else
    command_exists "$executable" || fail "executable not found on PATH: $executable"
  fi
}

normalize_base_path() {
  local path="$1"
  if [[ -z "$path" || "$path" == "/" ]]; then
    printf '\n'
    return
  fi
  path="/${path#/}"
  path="${path%/}"
  printf '%s\n' "$path"
}

stop_process() {
  local pid="$1"
  local killer_pid=""

  [[ -n "$pid" ]] || return 0
  kill -0 "$pid" 2>/dev/null || return 0

  kill "$pid" 2>/dev/null || true
  (
    sleep 5
    kill -KILL "$pid" 2>/dev/null || true
  ) &
  killer_pid=$!

  wait "$pid" 2>/dev/null || true
  kill "$killer_pid" 2>/dev/null || true
  wait "$killer_pid" 2>/dev/null || true
}

cleanup_children() {
  local status=$?

  trap - EXIT INT TERM
  stop_process "$stdio_pid"
  stop_process "$server_pid"

  return "$status"
}

wait_for_server() {
  local deadline
  deadline=$((SECONDS + ready_timeout))
  while (( SECONDS <= deadline )); do
    if [[ -n "$server_pid" ]] && ! kill -0 "$server_pid" 2>/dev/null; then
      if [[ -f "$server_log" ]]; then
        log "LeafWiki exited before readiness. Last server log lines:"
        tail -20 "$server_log" >&2 || true
      fi
      return 1
    fi
    if curl -fsS "$health_url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.2
  done

  log "Timed out waiting for LeafWiki at $health_url"
  if [[ -f "$server_log" ]]; then
    log "Last server log lines:"
    tail -20 "$server_log" >&2 || true
  fi
  return 1
}

while [[ $# -gt 0 ]]; do
  if [[ "$1" == --*" "* ]]; then
    fail "argument '$1' contains a space; MCP JSON args must split flags and values into separate args, for example \"--root-dir\", \"./wiki\", or use --root-dir=./wiki"
  fi

  case "$1" in
    --leafwiki-bin=*)
      leafwiki_bin="${1#*=}"
      shift
      ;;
    --leafwiki-bin)
      [[ $# -ge 2 ]] || fail "--leafwiki-bin requires a path"
      leafwiki_bin="$2"
      shift 2
      ;;
    --mcp-stdio-bin=*)
      mcp_stdio_bin="${1#*=}"
      shift
      ;;
    --mcp-stdio-bin)
      [[ $# -ge 2 ]] || fail "--mcp-stdio-bin requires a path"
      mcp_stdio_bin="$2"
      shift 2
      ;;
    --scheme=*)
      scheme="${1#*=}"
      shift
      ;;
    --scheme)
      [[ $# -ge 2 ]] || fail "--scheme requires a value"
      scheme="$2"
      shift 2
      ;;
    --host=*)
      host="${1#*=}"
      shift
      ;;
    --host)
      [[ $# -ge 2 ]] || fail "--host requires a value"
      host="$2"
      shift 2
      ;;
    --port=*)
      port="${1#*=}"
      shift
      ;;
    --port)
      [[ $# -ge 2 ]] || fail "--port requires a value"
      port="$2"
      shift 2
      ;;
    --base-path=*)
      base_path="${1#*=}"
      shift
      ;;
    --base-path)
      [[ $# -ge 2 ]] || fail "--base-path requires a path"
      base_path="$2"
      shift 2
      ;;
    --data-dir=*)
      data_dir="${1#*=}"
      shift
      ;;
    --data-dir)
      [[ $# -ge 2 ]] || fail "--data-dir requires a path"
      data_dir="$2"
      shift 2
      ;;
    --root-dir=*)
      root_dir="${1#*=}"
      shift
      ;;
    --root-dir)
      [[ $# -ge 2 ]] || fail "--root-dir requires a path"
      root_dir="$2"
      shift 2
      ;;
    --jwt-secret=*)
      jwt_secret="${1#*=}"
      shift
      ;;
    --jwt-secret)
      [[ $# -ge 2 ]] || fail "--jwt-secret requires a secret"
      jwt_secret="$2"
      shift 2
      ;;
    --admin-password=*)
      admin_password="${1#*=}"
      shift
      ;;
    --admin-password)
      [[ $# -ge 2 ]] || fail "--admin-password requires a password"
      admin_password="$2"
      shift 2
      ;;
    --disable-auth)
      disable_auth=1
      shift
      ;;
    --allow-insecure)
      allow_insecure=1
      shift
      ;;
    --no-allow-insecure)
      allow_insecure=0
      shift
      ;;
    --request-log)
      disable_request_log=0
      shift
      ;;
    --disable-request-log)
      disable_request_log=1
      shift
      ;;
    --endpoint=*)
      endpoint="${1#*=}"
      shift
      ;;
    --endpoint)
      [[ $# -ge 2 ]] || fail "--endpoint requires a URL"
      endpoint="$2"
      shift 2
      ;;
    --health-url=*)
      health_url="${1#*=}"
      shift
      ;;
    --health-url)
      [[ $# -ge 2 ]] || fail "--health-url requires a URL"
      health_url="$2"
      shift 2
      ;;
    --api-key=*)
      api_key="${1#*=}"
      shift
      ;;
    --api-key)
      [[ $# -ge 2 ]] || fail "--api-key requires a value"
      api_key="$2"
      shift 2
      ;;
    --request-timeout=*)
      request_timeout="${1#*=}"
      shift
      ;;
    --request-timeout)
      [[ $# -ge 2 ]] || fail "--request-timeout requires a duration"
      request_timeout="$2"
      shift 2
      ;;
    --shutdown-timeout=*)
      shutdown_timeout="${1#*=}"
      shift
      ;;
    --shutdown-timeout)
      [[ $# -ge 2 ]] || fail "--shutdown-timeout requires a duration"
      shutdown_timeout="$2"
      shift 2
      ;;
    --max-frame-size=*)
      max_frame_size="${1#*=}"
      shift
      ;;
    --max-frame-size)
      [[ $# -ge 2 ]] || fail "--max-frame-size requires a size"
      max_frame_size="$2"
      shift 2
      ;;
    --ready-timeout=*)
      ready_timeout="${1#*=}"
      shift
      ;;
    --ready-timeout)
      [[ $# -ge 2 ]] || fail "--ready-timeout requires seconds"
      ready_timeout="$2"
      shift 2
      ;;
    --server-log=*)
      server_log="${1#*=}"
      shift
      ;;
    --server-log)
      [[ $# -ge 2 ]] || fail "--server-log requires a path"
      server_log="$2"
      shift 2
      ;;
    --server-arg=*)
      server_extra_args+=("${1#*=}")
      shift
      ;;
    --server-arg)
      [[ $# -ge 2 ]] || fail "--server-arg requires an argument"
      server_extra_args+=("$2")
      shift 2
      ;;
    --stdio-arg=*)
      stdio_extra_args+=("${1#*=}")
      shift
      ;;
    --stdio-arg)
      [[ $# -ge 2 ]] || fail "--stdio-arg requires an argument"
      stdio_extra_args+=("$2")
      shift 2
      ;;
    --dry-run)
      dry_run=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown option: $1"
      ;;
  esac
done

[[ "$ready_timeout" =~ ^[0-9]+$ ]] || fail "--ready-timeout must be a non-negative integer"

base_path="$(normalize_base_path "$base_path")"
if [[ -z "$endpoint" ]]; then
  endpoint="$scheme://$host:$port$base_path/mcp"
fi
if [[ -z "$health_url" ]]; then
  health_url="$scheme://$host:$port$base_path/api/health"
fi

server_cmd=(
  "$leafwiki_bin"
  --enable-mcp
  --host "$host"
  --port "$port"
  --data-dir "$data_dir"
  --root-dir "$root_dir"
)
if truthy "$disable_auth"; then
  server_cmd+=(--disable-auth)
else
  server_cmd+=(--jwt-secret "$jwt_secret" --admin-password "$admin_password")
fi
if truthy "$allow_insecure"; then
  server_cmd+=(--allow-insecure)
fi
if [[ -n "$base_path" ]]; then
  server_cmd+=(--base-path "$base_path")
fi
if truthy "$disable_request_log"; then
  server_cmd+=(--disable-request-log)
fi
if [[ "${#server_extra_args[@]}" -gt 0 ]]; then
  server_cmd+=("${server_extra_args[@]}")
fi

stdio_cmd=("$mcp_stdio_bin" --endpoint "$endpoint")
if [[ -n "$api_key" ]]; then
  stdio_cmd+=(--api-key "$api_key")
fi
if [[ -n "$request_timeout" ]]; then
  stdio_cmd+=(--request-timeout "$request_timeout")
fi
if [[ -n "$shutdown_timeout" ]]; then
  stdio_cmd+=(--shutdown-timeout "$shutdown_timeout")
fi
if [[ -n "$max_frame_size" ]]; then
  stdio_cmd+=(--max-frame-size "$max_frame_size")
fi
if [[ "${#stdio_extra_args[@]}" -gt 0 ]]; then
  stdio_cmd+=("${stdio_extra_args[@]}")
fi

if [[ "$dry_run" -eq 1 ]]; then
  log "Would start LeafWiki and then run leafwiki-mcp-stdio"
  log "Health URL: $health_url"
  log "Server log: $server_log"
  print_command "${server_cmd[@]}"
  print_command "${stdio_cmd[@]}"
  exit 0
fi

require_executable "$leafwiki_bin"
require_executable "$mcp_stdio_bin"
command_exists curl || fail "curl is required to wait for LeafWiki readiness"

mkdir -p "$(dirname -- "$server_log")"
: > "$server_log"

trap cleanup_children EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

log "Starting LeafWiki. Server log: $server_log"
print_command "${server_cmd[@]}"
"${server_cmd[@]}" >> "$server_log" 2>&1 &
server_pid=$!

if [[ "$ready_timeout" -gt 0 ]]; then
  wait_for_server || exit 1
fi

log "Starting leafwiki-mcp-stdio for $endpoint"
print_command "${stdio_cmd[@]}"
set +e
"${stdio_cmd[@]}" <&0 &
stdio_pid=$!
wait "$stdio_pid"
stdio_status=$?
stdio_pid=""
set -e
exit "$stdio_status"
