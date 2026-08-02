// Isolates GET /api/tree's own concurrency behavior at a given page count.
// Built specifically to check whether the /api/tree unbounded-payload
// finding from the original 10k-page sweep (companion results page) is
// actually relevant at realistic wiki sizes (Patrick: "normally 1000-2000
// pages max"), rather than extrapolating from the 10k numbers.
//
// Run standalone, e.g.:
//   docker run --rm -i --network=host --user "$(id -u):$(id -g)" \
//     -v $(pwd)/loadtest/k6:/scripts -v $(pwd)/loadtest/seed:/seed \
//     -e BASE_URL=http://127.0.0.1:8091 -e SESSION_POOL_PATH=/seed/.session-pool.json \
//     -e VUS=10 -e STAGE_DURATION=15s \
//     grafana/k6 run /scripts/tree-only.js
import http from 'k6/http';
import { check } from 'k6';
import { Trend } from 'k6/metrics';
import { pickSession, authHeaders } from './lib/auth.js';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8091';
const SESSION_POOL_PATH = __ENV.SESSION_POOL_PATH || '/seed/.session-pool.json';
const VUS = parseInt(__ENV.VUS || '10', 10);
const STAGE_DURATION = __ENV.STAGE_DURATION || '15s';

const pool = JSON.parse(open(SESSION_POOL_PATH));

const treeDuration = new Trend('tree_duration_ms', true);

export const options = {
  scenarios: {
    treereaders: {
      executor: 'constant-vus',
      vus: VUS,
      duration: STAGE_DURATION,
    },
  },
};

export default function () {
  const session = pickSession(pool, __VU);
  const res = http.get(`${BASE_URL}/api/tree`, { headers: authHeaders(session) });
  treeDuration.add(res.timings.duration);
  check(res, { 'tree ok': (r) => r.status === 200 });
}
