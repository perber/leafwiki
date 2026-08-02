// Isolates GET /api/search's own concurrency behavior — no writers, no
// page reads, just repeated search queries — to tell apart two different
// possible causes for its baseline sluggishness (454-594ms observed at 10
// concurrent readers doing search+page-view together, see
// readers-writers-during-refactor results): reader-self-contention behind
// SQLiteIndex's plain sync.Mutex (same signature links had before its
// RWMutex fix) vs. genuinely expensive per-query FTS5 cost that would need
// a different fix entirely.
//
// Run standalone, e.g.:
//   docker run --rm -i --network=host --user "$(id -u):$(id -g)" \
//     -v $(pwd)/loadtest/k6:/scripts -v $(pwd)/loadtest/seed:/seed \
//     -e BASE_URL=http://127.0.0.1:8091 -e SESSION_POOL_PATH=/seed/.session-pool.json \
//     -e VUS=10 -e STAGE_DURATION=15s \
//     grafana/k6 run /scripts/search-only.js
import http from 'k6/http';
import { check } from 'k6';
import { Trend } from 'k6/metrics';
import { pickSession, authHeaders } from './lib/auth.js';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8091';
const SESSION_POOL_PATH = __ENV.SESSION_POOL_PATH || '/seed/.session-pool.json';
const VUS = parseInt(__ENV.VUS || '10', 10);
const STAGE_DURATION = __ENV.STAGE_DURATION || '15s';
const SEARCH_TERM = __ENV.SEARCH_TERM || 'Seed';

const pool = JSON.parse(open(SESSION_POOL_PATH));

const searchDuration = new Trend('search_duration_ms', true);

export const options = {
  scenarios: {
    searchers: {
      executor: 'constant-vus',
      vus: VUS,
      duration: STAGE_DURATION,
    },
  },
};

export default function () {
  const session = pickSession(pool, __VU);
  const res = http.get(`${BASE_URL}/api/search?q=${SEARCH_TERM}`, { headers: authHeaders(session) });
  searchDuration.add(res.timings.duration);
  check(res, { 'search ok': (r) => r.status === 200 });
}
