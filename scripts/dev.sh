#!/usr/bin/env bash
# Runs the backend (:8080) and the Vite dev server (:5173) together; Ctrl-C
# stops both. Open http://localhost:5173, Vite proxies /api, /assets and
# /branding to the backend. Login: admin / devpassword. Wiki data lands in
# cmd/leafwiki/data (gitignored). Arguments go to the backend, e.g.
# `make dev ARGS=--enable-revision=true`. bash, not sh: dash turns job
# control (set -m below) off without a terminal and would leave Vite running.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
UI_DIR="$ROOT/ui/leafwiki-ui"

for tool in go npm; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        echo "$tool not found; 'make doctor' shows what is missing." >&2
        exit 1
    fi
done

if [ ! -d "$UI_DIR/node_modules" ]; then
    echo "Frontend dependencies are missing; run 'make setup' first." >&2
    exit 1
fi

# Build instead of `go run`: signalling `go run` reaches only the wrapper, not
# the server it starts (see e2e/run.sh), and compile errors show up before
# Vite starts.
tmp="$(mktemp -d)"
bin="$tmp/leafwiki$(cd "$ROOT" && go env GOEXE)"
echo "Building the backend..."
if ! (cd "$ROOT" && go build -o "$bin" ./cmd/leafwiki); then
    rm -rf "$tmp"
    exit 1
fi

backend=""
frontend=""
cleanup() {
    trap - EXIT INT TERM
    for pid in $frontend $backend; do
        # The whole process group (npm -> vite -> esbuild), else the process.
        kill -TERM -- "-$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
    done
    for pid in $frontend $backend; do
        wait "$pid" 2>/dev/null || true
    done
    rm -rf "$tmp"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# Job control puts each background job into its own process group, so cleanup
# can stop everything it spawned. Off again before the wait loop: with it on,
# the loop's foreground `sleep` would get the terminal and with it Ctrl-C.
# Both read /dev/null, a background process group reading the terminal stops.
set -m
(cd "$ROOT/cmd/leafwiki" && exec "$bin" --jwt-secret=dev --admin-password=devpassword --public-access=true --allow-insecure=true "$@") </dev/null &
backend=$!
(cd "$UI_DIR" && exec npm run dev) </dev/null &
frontend=$!
set +m

echo "Backend on http://localhost:8080, frontend on http://localhost:5173 (Ctrl-C stops both)."
while kill -0 "$backend" 2>/dev/null && kill -0 "$frontend" 2>/dev/null; do
    sleep 1
done
echo "The backend or the frontend exited; stopping the other." >&2
exit 1
