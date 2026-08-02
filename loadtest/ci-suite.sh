#!/usr/bin/env bash
# ci-suite.sh — self-contained load-test smoke suite: builds a throwaway
# instance, seeds a small dataset, runs a curated set of k6 scenarios at
# CI-appropriate (short, light) parameters, generates a report via
# ci-report.py, and exits non-zero on errors or a regression vs. the
# checked-in baseline (loadtest/ci-baseline.json).
#
# Not wired into any CI system yet — that's the deliberate next step, not
# this one. Until then, run it by hand the same way CI eventually will:
#
#   ./loadtest/ci-suite.sh
#
# First run (no baseline yet, or after an intentional performance change):
#
#   ./loadtest/ci-suite.sh --update-baseline
#
# Unlike run.sh (which assumes an already-running instance you manage
# yourself, for deep manual sweeps), this script owns the full lifecycle —
# build, seed, start, test, stop, clean up — because a CI job can't rely on
# anything being pre-provisioned.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

PORT="${PORT:-8093}"
METRICS_PORT="${METRICS_PORT:-9093}"
BASE_URL="http://127.0.0.1:${PORT}"
DATA_DIR="${DATA_DIR:-$(mktemp -d /tmp/leafwiki-ci-suite-XXXXXX)}"
BINARY="${BINARY:-$(mktemp -u /tmp/leafwiki-ci-suite-bin-XXXXXX)}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-CI-Suite-Admin-Pw-1!}"
RESULTS_DIR="${RESULTS_DIR:-$SCRIPT_DIR/results/ci/$(date +%Y%m%d-%H%M%S)}"
SCENARIO_DURATION="${SCENARIO_DURATION:-8s}"
UPDATE_BASELINE=0
KEEP_DATA=0

for arg in "$@"; do
  case "$arg" in
    --update-baseline) UPDATE_BASELINE=1 ;;
    --keep-data) KEEP_DATA=1 ;;
    *) echo "ci-suite.sh: unknown argument $arg" >&2; exit 1 ;;
  esac
done

SERVER_PID=""
cleanup() {
  if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  if [ "$KEEP_DATA" -eq 0 ]; then
    rm -rf "$DATA_DIR" "$BINARY"
  else
    echo "ci-suite.sh: --keep-data set, leaving $DATA_DIR and $BINARY in place"
  fi
}
trap cleanup EXIT

mkdir -p "$RESULTS_DIR"
echo "ci-suite.sh: results -> $RESULTS_DIR"

echo "ci-suite.sh: building..."
(cd "$REPO_ROOT" && go build -o "$BINARY" ./cmd/leafwiki)

echo "ci-suite.sh: seeding (500 flat pages + 1 section of 20 for sort)..."
(cd "$REPO_ROOT" && go run ./loadtest/seed/gen-pages --count 500 --dir "$DATA_DIR" >/dev/null)
(cd "$REPO_ROOT" && go run ./loadtest/seed/gen-nested --dir "$DATA_DIR" --sections 1 --pages-per-section 20 --start-index 1 >/dev/null)

echo "ci-suite.sh: starting server on $BASE_URL ..."
"$BINARY" \
  --data-dir "$DATA_DIR" \
  --host 127.0.0.1 --port "$PORT" \
  --jwt-secret "ci-suite-jwt-secret-not-for-prod" \
  --admin-username admin --admin-email admin@localhost \
  --admin-password "$ADMIN_PASSWORD" \
  --allow-insecure \
  --enable-metrics --metrics-host 127.0.0.1 --metrics-port "$METRICS_PORT" \
  > "$RESULTS_DIR/server.log" 2>&1 &
SERVER_PID=$!

for i in $(seq 1 30); do
  if curl -sf -o /dev/null "$BASE_URL/api/health"; then
    echo "ci-suite.sh: healthy after ${i}s"
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "ci-suite.sh: server did not become healthy in time" >&2
    cat "$RESULTS_DIR/server.log" >&2
    exit 1
  fi
  sleep 1
done

echo "ci-suite.sh: creating session pool..."
BASE_URL="$BASE_URL" ADMIN_IDENTIFIER=admin ADMIN_PASSWORD="$ADMIN_PASSWORD" USER_COUNT=10 \
  "$SCRIPT_DIR/seed/create-users.sh" >/dev/null

# run_k6 <script> <name> [-e KEY=VAL ...] -- [k6 CLI args, e.g. --vus 10 --duration 8s]
# The "--" separator is required because most scripts here take VUS/
# STAGE_DURATION as env vars (options.scenarios), but write-different-pages.js
# predates that convention and still expects --vus/--duration on the k6 CLI.
run_k6() {
  local script="$1"; shift
  local name="$1"; shift
  local env_args=()
  while [ "$1" != "--" ]; do
    env_args+=("$1")
    shift
  done
  shift # consume "--"
  docker run --rm -i --network=host --user "$(id -u):$(id -g)" \
    -v "$SCRIPT_DIR/k6:/scripts" -v "$SCRIPT_DIR/seed:/seed" \
    -e BASE_URL="$BASE_URL" -e SESSION_POOL_PATH=/seed/.session-pool.json \
    "${env_args[@]}" \
    grafana/k6 run "/scripts/$script" \
    --summary-export=/scripts/.tmp-ci-summary.json \
    "$@" \
    > "$RESULTS_DIR/$name.log" 2>&1
  mv "$SCRIPT_DIR/k6/.tmp-ci-summary.json" "$RESULTS_DIR/$name.summary.json"
  echo "ci-suite.sh: ran $name"
}

echo "ci-suite.sh: running scenarios (duration=$SCENARIO_DURATION each)..."
run_k6 write-different-pages.js write-different-pages \
  -e PAGE_COUNT=500 -- --vus 10 --duration "$SCENARIO_DURATION" || true
run_k6 tree-only.js tree-only \
  -e VUS=10 -e STAGE_DURATION="$SCENARIO_DURATION" -- || true
run_k6 search-only.js search-only \
  -e VUS=5 -e STAGE_DURATION="$SCENARIO_DURATION" -- || true
run_k6 readers-during-writers.js readers-content \
  -e READER_MODE=content -e READER_VUS=20 -e WRITER_VUS=0 -e PAGE_COUNT=500 -e STAGE_DURATION="$SCENARIO_DURATION" -- || true
run_k6 create-pages.js create-pages \
  -e VUS=5 -e STAGE_DURATION="$SCENARIO_DURATION" -e RUN_ID=ci -- || true
run_k6 copy-pages.js copy-pages \
  -e VUS=5 -e STAGE_DURATION="$SCENARIO_DURATION" -e RUN_ID=ci -e SOURCE_ID=page-00001 -- || true
run_k6 pin-pages.js pin-pages \
  -e VUS=5 -e STAGE_DURATION="$SCENARIO_DURATION" -e PAGE_COUNT=500 -- || true
run_k6 convert-pages.js convert-pages \
  -e VUS=5 -e STAGE_DURATION="$SCENARIO_DURATION" -e PAGE_COUNT=500 -- || true
run_k6 sort-pages.js sort-pages \
  -e VUS=1 -e STAGE_DURATION="$SCENARIO_DURATION" -e SECTION_START=1 -- || true

echo ""
REPORT_ARGS=("$RESULTS_DIR")
if [ "$UPDATE_BASELINE" -eq 1 ]; then
  REPORT_ARGS+=(--update-baseline)
fi
python3 "$SCRIPT_DIR/ci-report.py" "${REPORT_ARGS[@]}"
