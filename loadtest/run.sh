#!/usr/bin/env bash
# run.sh — concurrency sweep orchestration for the LeafWiki edit-capacity
# load test. Assumes a throwaway LeafWiki instance is already running with
# --enable-metrics (see loadtest/README.md) and loadtest/seed/.session-pool.json
# already exists (via create-users.sh). This script does NOT start/stop the
# server or seed data — keeping that manual keeps the blast radius obvious
# (you always know exactly which instance/data-dir you're hitting).
#
# For each scenario (write-different-pages, write-same-page, read-paths) and
# each VU level in VU_LEVELS, runs k6 via Docker for STAGE_DURATION, saves
# its JSON summary, and snapshots /metrics before+after so side-effect costs
# can be attributed later.
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8091}"
METRICS_URL="${METRICS_URL:-http://127.0.0.1:9091/metrics}"
VU_LEVELS="${VU_LEVELS:-1 5 10 20 40 80}"
STAGE_DURATION="${STAGE_DURATION:-20s}"
SCENARIOS="${SCENARIOS:-write-different-pages write-same-page read-paths}"
TARGET_PAGE_ID="${TARGET_PAGE_ID:-page-00001}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SESSION_POOL_PATH="${SESSION_POOL_PATH:-/seed/.session-pool.json}"
RESULTS_DIR="${RESULTS_DIR:-$SCRIPT_DIR/results/$(date +%Y%m%d-%H%M%S)}"

if [ ! -f "$SCRIPT_DIR/seed/.session-pool.json" ]; then
  echo "run.sh: $SCRIPT_DIR/seed/.session-pool.json not found — run create-users.sh first" >&2
  exit 1
fi

mkdir -p "$RESULTS_DIR"
echo "run.sh: results -> $RESULTS_DIR"

echo "run.sh: checking server health at $BASE_URL ..."
if ! curl -sf -o /dev/null "$BASE_URL/api/health"; then
  echo "run.sh: server not reachable at $BASE_URL/api/health" >&2
  exit 1
fi

snapshot_metrics() {
  local label="$1"
  curl -sf "$METRICS_URL" -o "$RESULTS_DIR/metrics.$label.prom" 2>/dev/null || \
    echo "run.sh: WARN could not scrape $METRICS_URL (continuing without it)" >&2
}

for scenario in $SCENARIOS; do
  for vus in $VU_LEVELS; do
    run_id="${scenario}-vus${vus}"
    echo ""
    echo "run.sh: === $run_id (duration=$STAGE_DURATION) ==="

    snapshot_metrics "before.$run_id"

    docker run --rm -i --network=host --user "$(id -u):$(id -g)" \
      -v "$SCRIPT_DIR/k6:/scripts" -v "$SCRIPT_DIR/seed:/seed" \
      -e BASE_URL="$BASE_URL" \
      -e SESSION_POOL_PATH="$SESSION_POOL_PATH" \
      -e TARGET_PAGE_ID="$TARGET_PAGE_ID" \
      grafana/k6 run "/scripts/${scenario}.js" \
      --vus "$vus" --duration "$STAGE_DURATION" \
      --summary-export=/scripts/.tmp-summary.json \
      2>&1 | tee "$RESULTS_DIR/$run_id.log"

    if [ -f "$SCRIPT_DIR/k6/.tmp-summary.json" ]; then
      mv "$SCRIPT_DIR/k6/.tmp-summary.json" "$RESULTS_DIR/$run_id.summary.json"
    fi

    snapshot_metrics "after.$run_id"
  done
done

echo ""
echo "run.sh: sweep complete. Results in $RESULTS_DIR"
