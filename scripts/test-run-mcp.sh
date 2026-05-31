#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/.." && pwd)"
script="$repo_root/scripts/run-mcp.sh"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local label="$3"
  [[ "$haystack" == *"$needle"* ]] || fail "$label missing '$needle': $haystack"
}

wait_for_file() {
  local path="$1"
  local label="$2"
  local attempt
  for attempt in {1..50}; do
    [[ -s "$path" ]] && return 0
    sleep 0.1
  done
  fail "timed out waiting for $label at $path"
}

wait_for_process_exit() {
  local pid="$1"
  local label="$2"
  local attempt
  for attempt in {1..50}; do
    if ! kill -0 "$pid" 2>/dev/null; then
      return 0
    fi
    sleep 0.1
  done
  fail "$label process $pid did not exit"
}

[[ -f "$script" ]] || fail "missing scripts/run-mcp.sh"

bash -n "$script"

help_output="$("$script" --help)"
assert_contains "$help_output" "--leafwiki-bin" "help"
assert_contains "$help_output" "--mcp-stdio-bin" "help"
assert_contains "$help_output" "--endpoint" "help"
assert_contains "$help_output" "--api-key" "help"
assert_contains "$help_output" "--server-arg" "help"
assert_contains "$help_output" "--stdio-arg" "help"
assert_contains "$help_output" "--dry-run" "help"

tmp_dir="$(mktemp -d)"
cleanup() {
  local pid
  for pid in "${wrapper_pid:-}" "${long_stdio_pid:-}" "${long_leafwiki_pid:-}"; do
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      sleep 0.1
      kill -KILL "$pid" 2>/dev/null || true
    fi
  done
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

equals_output="$(
  "$script" \
    --dry-run \
    --leafwiki-bin=/tmp/fake-leafwiki \
    --mcp-stdio-bin=/tmp/fake-stdio \
    --host=127.0.0.1 \
    --port=18082 \
    --root-dir="$tmp_dir/wiki-equals" \
    --data-dir="$tmp_dir/data-equals" \
    --jwt-secret=test-secret \
    --admin-password=admin \
    --api-key=lwk_equals_secret \
    --server-log="$tmp_dir/server-equals.log" \
    2>&1
)"

assert_contains "$equals_output" "--root-dir $tmp_dir/wiki-equals" "equals dry-run"
assert_contains "$equals_output" "--endpoint http://127.0.0.1:18082/mcp" "equals dry-run"
assert_contains "$equals_output" "--api-key lwk_equals_secret" "equals dry-run"

set +e
bad_combined_arg_output="$("$script" --dry-run "--root-dir ./wiki" 2>&1)"
bad_combined_arg_status=$?
set -e
[[ "$bad_combined_arg_status" -ne 0 ]] || fail "combined flag/value argument unexpectedly succeeded"
assert_contains "$bad_combined_arg_output" "split flags and values into separate args" "combined arg error"

dry_run_output="$(
  "$script" \
    --dry-run \
    --leafwiki-bin /tmp/fake-leafwiki \
    --mcp-stdio-bin /tmp/fake-stdio \
    --host 127.0.0.1 \
    --port 18081 \
    --root-dir "$tmp_dir/wiki" \
    --data-dir "$tmp_dir/data" \
    --jwt-secret test-secret \
    --admin-password admin \
    --api-key lwk_test_secret \
    --server-log "$tmp_dir/server.log" \
    2>&1
)"

assert_contains "$dry_run_output" "Would start LeafWiki and then run leafwiki-mcp-stdio" "dry-run"
assert_contains "$dry_run_output" "/tmp/fake-leafwiki" "dry-run"
assert_contains "$dry_run_output" "--enable-mcp" "dry-run"
assert_contains "$dry_run_output" "--root-dir $tmp_dir/wiki" "dry-run"
assert_contains "$dry_run_output" "--jwt-secret test-secret" "dry-run"
assert_contains "$dry_run_output" "/tmp/fake-stdio" "dry-run"
assert_contains "$dry_run_output" "--endpoint http://127.0.0.1:18081/mcp" "dry-run"
assert_contains "$dry_run_output" "--api-key lwk_test_secret" "dry-run"
[[ ! -e "$tmp_dir/server.log" ]] || fail "dry-run created server log"

fake_bin="$tmp_dir/bin"
mkdir -p "$fake_bin"

cat > "$fake_bin/leafwiki" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

port="8080"
printf '%s\n' "$@" > "$FAKE_LEAFWIKI_ARGS"
if [[ -n "${FAKE_LEAFWIKI_PID:-}" ]]; then
  printf '%s\n' "$$" > "$FAKE_LEAFWIKI_PID"
fi
while [[ $# -gt 0 ]]; do
  case "$1" in
    --port)
      port="$2"
      shift 2
      ;;
    --port=*)
      port="${1#--port=}"
      shift
      ;;
    *)
      shift
      ;;
  esac
done

exec python3 - "$port" <<'PY'
import http.server
import socketserver
import sys

port = int(sys.argv[1])

class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/api/health":
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"ok")
        else:
            self.send_response(404)
            self.end_headers()

    def log_message(self, format, *args):
        pass

with socketserver.TCPServer(("127.0.0.1", port), Handler) as server:
    server.serve_forever()
PY
EOF

cat > "$fake_bin/leafwiki-mcp-stdio" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" > "$FAKE_STDIO_ARGS"
if [[ "${FAKE_STDIO_MODE:-}" == "wait" ]]; then
  printf '%s\n' "$$" > "$FAKE_STDIO_PID"
  trap 'printf "terminated\n" > "$FAKE_STDIO_TERM_FILE"; exit 143' TERM INT
  trap '' HUP
  while :; do
    sleep 1
  done
fi
printf 'stdio stdout\n'
printf 'stdio stderr\n' >&2
EOF

chmod +x "$fake_bin/leafwiki" "$fake_bin/leafwiki-mcp-stdio"

port="$(python3 - <<'PY'
import socket
with socket.socket() as s:
    s.bind(("127.0.0.1", 0))
    print(s.getsockname()[1])
PY
)"

stdout_file="$tmp_dir/stdout"
stderr_file="$tmp_dir/stderr"
leafwiki_args_file="$tmp_dir/leafwiki.args"
stdio_args_file="$tmp_dir/stdio.args"

FAKE_LEAFWIKI_ARGS="$leafwiki_args_file" \
FAKE_STDIO_ARGS="$stdio_args_file" \
"$script" \
  --leafwiki-bin "$fake_bin/leafwiki" \
  --mcp-stdio-bin "$fake_bin/leafwiki-mcp-stdio" \
  --host 127.0.0.1 \
  --port "$port" \
  --root-dir "$tmp_dir/wiki" \
  --data-dir "$tmp_dir/data" \
  --jwt-secret test-secret \
  --admin-password admin \
  --api-key lwk_test_secret \
  --server-log "$tmp_dir/server.log" \
  --ready-timeout 5 \
  > "$stdout_file" \
  2> "$stderr_file"

[[ "$(cat "$stdout_file")" == "stdio stdout" ]] || fail "stdout was not reserved for stdio proxy: $(cat "$stdout_file")"
assert_contains "$(cat "$stderr_file")" "stdio stderr" "stderr"
assert_contains "$(cat "$leafwiki_args_file")" "--enable-mcp" "leafwiki args"
assert_contains "$(cat "$leafwiki_args_file")" "--host" "leafwiki args"
assert_contains "$(cat "$leafwiki_args_file")" "127.0.0.1" "leafwiki args"
assert_contains "$(cat "$leafwiki_args_file")" "--port" "leafwiki args"
assert_contains "$(cat "$leafwiki_args_file")" "$port" "leafwiki args"
assert_contains "$(cat "$leafwiki_args_file")" "--root-dir" "leafwiki args"
assert_contains "$(cat "$leafwiki_args_file")" "$tmp_dir/wiki" "leafwiki args"
assert_contains "$(cat "$leafwiki_args_file")" "--jwt-secret" "leafwiki args"
assert_contains "$(cat "$leafwiki_args_file")" "test-secret" "leafwiki args"
assert_contains "$(cat "$stdio_args_file")" "--endpoint" "stdio args"
assert_contains "$(cat "$stdio_args_file")" "http://127.0.0.1:$port/mcp" "stdio args"
assert_contains "$(cat "$stdio_args_file")" "--api-key" "stdio args"
assert_contains "$(cat "$stdio_args_file")" "lwk_test_secret" "stdio args"

long_port="$(python3 - <<'PY'
import socket
with socket.socket() as s:
    s.bind(("127.0.0.1", 0))
    print(s.getsockname()[1])
PY
)"

long_leafwiki_pid_file="$tmp_dir/long-leafwiki.pid"
long_stdio_pid_file="$tmp_dir/long-stdio.pid"
long_stdio_term_file="$tmp_dir/long-stdio.term"
long_stdout_file="$tmp_dir/long-stdout"
long_stderr_file="$tmp_dir/long-stderr"

FAKE_LEAFWIKI_ARGS="$tmp_dir/long-leafwiki.args" \
FAKE_LEAFWIKI_PID="$long_leafwiki_pid_file" \
FAKE_STDIO_ARGS="$tmp_dir/long-stdio.args" \
FAKE_STDIO_MODE=wait \
FAKE_STDIO_PID="$long_stdio_pid_file" \
FAKE_STDIO_TERM_FILE="$long_stdio_term_file" \
"$script" \
  --leafwiki-bin "$fake_bin/leafwiki" \
  --mcp-stdio-bin "$fake_bin/leafwiki-mcp-stdio" \
  --host 127.0.0.1 \
  --port "$long_port" \
  --root-dir "$tmp_dir/wiki-long" \
  --data-dir "$tmp_dir/data-long" \
  --jwt-secret test-secret \
  --admin-password admin \
  --api-key lwk_test_secret \
  --server-log "$tmp_dir/long-server.log" \
  --ready-timeout 5 \
  > "$long_stdout_file" \
  2> "$long_stderr_file" &
wrapper_pid=$!

wait_for_file "$long_leafwiki_pid_file" "long LeafWiki pid"
wait_for_file "$long_stdio_pid_file" "long stdio pid"
long_leafwiki_pid="$(cat "$long_leafwiki_pid_file")"
long_stdio_pid="$(cat "$long_stdio_pid_file")"

kill -TERM "$wrapper_pid"
wait_for_process_exit "$wrapper_pid" "run-mcp wrapper"
wait_for_process_exit "$long_stdio_pid" "leafwiki-mcp-stdio child"
wait_for_process_exit "$long_leafwiki_pid" "LeafWiki child"
[[ -f "$long_stdio_term_file" ]] || fail "leafwiki-mcp-stdio child did not receive termination"

printf 'PASS: run-mcp script checks\n'
