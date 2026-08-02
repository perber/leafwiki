#!/usr/bin/env bash
# run-reads-during-user-writes.sh — measure-only load test for
# internal/core/auth/user_store.go (see the plan: this store is on the
# request path of EVERY authenticated call, via RequireAuth ->
# ValidateToken -> GetUserByID, so its concurrency behavior matters more
# broadly than any single feature's SQLite store).
#
# Assumes a throwaway LeafWiki instance is already running with
# --enable-metrics, loadtest/seed/.session-pool.json exists (create-users.sh),
# and loadtest/seed/.admin-session.json + .writer-accounts.json exist
# (create-writer-accounts.sh).
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8091}"
METRICS_URL="${METRICS_URL:-http://127.0.0.1:9091/metrics}"
READER_VU_LEVELS="${READER_VU_LEVELS:-1 5 10 20 40 80}"
WRITER_VU_LEVELS="${WRITER_VU_LEVELS:-0 3}"
STAGE_DURATION="${STAGE_DURATION:-20s}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SESSION_POOL_PATH="${SESSION_POOL_PATH:-/seed/.session-pool.json}"
ADMIN_SESSION_PATH="${ADMIN_SESSION_PATH:-/seed/.admin-session.json}"
WRITER_ACCOUNTS_PATH="${WRITER_ACCOUNTS_PATH:-/seed/.writer-accounts.json}"
RESULTS_DIR="${RESULTS_DIR:-$SCRIPT_DIR/results/$(date +%Y%m%d-%H%M%S)-reads-during-user-writes}"

if [ ! -f "$SCRIPT_DIR/seed/.session-pool.json" ]; then
  echo "run-reads-during-user-writes.sh: .session-pool.json not found — run create-users.sh first" >&2
  exit 1
fi
if [ ! -f "$SCRIPT_DIR/seed/.admin-session.json" ] || [ ! -f "$SCRIPT_DIR/seed/.writer-accounts.json" ]; then
  echo "run-reads-during-user-writes.sh: admin session / writer accounts not found — run create-writer-accounts.sh first" >&2
  exit 1
fi

mkdir -p "$RESULTS_DIR"
echo "run-reads-during-user-writes.sh: results -> $RESULTS_DIR"

echo "run-reads-during-user-writes.sh: checking server health at $BASE_URL ..."
if ! curl -sf -o /dev/null "$BASE_URL/api/health"; then
  echo "run-reads-during-user-writes.sh: server not reachable at $BASE_URL/api/health" >&2
  exit 1
fi

snapshot_metrics() {
  local label="$1"
  curl -sf "$METRICS_URL" -o "$RESULTS_DIR/metrics.$label.prom" 2>/dev/null || \
    echo "run-reads-during-user-writes.sh: WARN could not scrape $METRICS_URL (continuing without it)" >&2
}

for writer_vus in $WRITER_VU_LEVELS; do
  for reader_vus in $READER_VU_LEVELS; do
    run_id="reads-during-user-writes-w${writer_vus}-r${reader_vus}"
    echo ""
    echo "run-reads-during-user-writes.sh: === $run_id (duration=$STAGE_DURATION) ==="

    snapshot_metrics "before.$run_id"

    docker run --rm -i --network=host --user "$(id -u):$(id -g)" \
      -v "$SCRIPT_DIR/k6:/scripts" -v "$SCRIPT_DIR/seed:/seed" \
      -e BASE_URL="$BASE_URL" \
      -e SESSION_POOL_PATH="$SESSION_POOL_PATH" \
      -e ADMIN_SESSION_PATH="$ADMIN_SESSION_PATH" \
      -e WRITER_ACCOUNTS_PATH="$WRITER_ACCOUNTS_PATH" \
      -e READER_VUS="$reader_vus" \
      -e WRITER_VUS="$writer_vus" \
      -e STAGE_DURATION="$STAGE_DURATION" \
      grafana/k6 run /scripts/reads-during-user-writes.js \
      --summary-export=/scripts/.tmp-summary.json \
      2>&1 | tee "$RESULTS_DIR/$run_id.log"

    if [ -f "$SCRIPT_DIR/k6/.tmp-summary.json" ]; then
      mv "$SCRIPT_DIR/k6/.tmp-summary.json" "$RESULTS_DIR/$run_id.summary.json"
    fi

    snapshot_metrics "after.$run_id"
  done
done

echo ""
echo "run-reads-during-user-writes.sh: sweep complete. Results in $RESULTS_DIR"
