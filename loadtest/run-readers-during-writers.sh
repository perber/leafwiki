#!/usr/bin/env bash
# run-readers-during-writers.sh — Scenario 1 from
# plans/loadtest-additional-scenarios.md: reader latency at 0 vs. N
# concurrent writers, across a reader VU sweep.
#
# k6/readers-during-writers.js defines its own `options.scenarios` (readers
# always on, writers only added when WRITER_VUS>0) instead of the
# single-scenario --vus/--duration flags run.sh's scenarios use, so VU
# counts and duration are passed as env vars here, and this gets its own
# driver script rather than folding into run.sh's SCENARIOS loop.
#
# Assumes a throwaway LeafWiki instance is already running with
# --enable-metrics (see loadtest/README.md) and
# loadtest/seed/.session-pool.json already exists (via create-users.sh).
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8091}"
METRICS_URL="${METRICS_URL:-http://127.0.0.1:9091/metrics}"
READER_VU_LEVELS="${READER_VU_LEVELS:-1 5 10 20 40 80}"
WRITER_VU_LEVELS="${WRITER_VU_LEVELS:-0 10}"
STAGE_DURATION="${STAGE_DURATION:-20s}"
READER_MODE="${READER_MODE:-content}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SESSION_POOL_PATH="${SESSION_POOL_PATH:-/seed/.session-pool.json}"
RESULTS_DIR="${RESULTS_DIR:-$SCRIPT_DIR/results/$(date +%Y%m%d-%H%M%S)-readers-during-writers-${READER_MODE}}"

if [ ! -f "$SCRIPT_DIR/seed/.session-pool.json" ]; then
  echo "run-readers-during-writers.sh: $SCRIPT_DIR/seed/.session-pool.json not found — run create-users.sh first" >&2
  exit 1
fi

mkdir -p "$RESULTS_DIR"
echo "run-readers-during-writers.sh: results -> $RESULTS_DIR"

echo "run-readers-during-writers.sh: checking server health at $BASE_URL ..."
if ! curl -sf -o /dev/null "$BASE_URL/api/health"; then
  echo "run-readers-during-writers.sh: server not reachable at $BASE_URL/api/health" >&2
  exit 1
fi

snapshot_metrics() {
  local label="$1"
  curl -sf "$METRICS_URL" -o "$RESULTS_DIR/metrics.$label.prom" 2>/dev/null || \
    echo "run-readers-during-writers.sh: WARN could not scrape $METRICS_URL (continuing without it)" >&2
}

for writer_vus in $WRITER_VU_LEVELS; do
  for reader_vus in $READER_VU_LEVELS; do
    run_id="readers-during-writers-w${writer_vus}-r${reader_vus}"
    echo ""
    echo "run-readers-during-writers.sh: === $run_id (duration=$STAGE_DURATION) ==="

    snapshot_metrics "before.$run_id"

    docker run --rm -i --network=host --user "$(id -u):$(id -g)" \
      -v "$SCRIPT_DIR/k6:/scripts" -v "$SCRIPT_DIR/seed:/seed" \
      -e BASE_URL="$BASE_URL" \
      -e SESSION_POOL_PATH="$SESSION_POOL_PATH" \
      -e READER_VUS="$reader_vus" \
      -e WRITER_VUS="$writer_vus" \
      -e STAGE_DURATION="$STAGE_DURATION" \
      -e READER_MODE="$READER_MODE" \
      grafana/k6 run /scripts/readers-during-writers.js \
      --summary-export=/scripts/.tmp-summary.json \
      2>&1 | tee "$RESULTS_DIR/$run_id.log"

    if [ -f "$SCRIPT_DIR/k6/.tmp-summary.json" ]; then
      mv "$SCRIPT_DIR/k6/.tmp-summary.json" "$RESULTS_DIR/$run_id.summary.json"
    fi

    snapshot_metrics "after.$run_id"
  done
done

echo ""
echo "run-readers-during-writers.sh: sweep complete. Results in $RESULTS_DIR"
