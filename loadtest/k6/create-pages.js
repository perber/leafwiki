// Create sweep — each VU repeatedly creates brand-new root-level pages with
// a unique slug (never revisited, so no conflicts, no seed data needed
// beyond a valid session pool). Isolates CreatePageUseCase/tree-insert cost
// under concurrency, independent of any pre-existing page count.
//
// Run standalone, e.g.:
//   docker run --rm -i --network=host --user "$(id -u):$(id -g)" \
//     -v $(pwd)/loadtest/k6:/scripts -v $(pwd)/loadtest/seed:/seed \
//     -e BASE_URL=http://127.0.0.1:8091 -e SESSION_POOL_PATH=/seed/.session-pool.json \
//     -e VUS=10 -e STAGE_DURATION=15s \
//     grafana/k6 run /scripts/create-pages.js
import http from 'k6/http';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { pickSession, writeHeaders } from './lib/auth.js';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8091';
const SESSION_POOL_PATH = __ENV.SESSION_POOL_PATH || '/seed/.session-pool.json';
const VUS = parseInt(__ENV.VUS || '10', 10);
const STAGE_DURATION = __ENV.STAGE_DURATION || '15s';
const RUN_ID = __ENV.RUN_ID || String(Date.now());

const pool = JSON.parse(open(SESSION_POOL_PATH));

const createDuration = new Trend('create_duration_ms', true);
const unexpectedErrors = new Counter('unexpected_errors');

export const options = {
  scenarios: {
    creators: {
      executor: 'constant-vus',
      vus: VUS,
      duration: STAGE_DURATION,
    },
  },
  thresholds: {
    unexpected_errors: ['count==0'],
  },
};

export default function () {
  const session = pickSession(pool, __VU);
  const slug = `load-create-${RUN_ID}-vu${__VU}-${__ITER}`;

  const payload = JSON.stringify({
    parentId: null,
    title: `Load Create ${RUN_ID} VU${__VU}/${__ITER}`,
    slug,
    kind: 'page',
  });

  const res = http.post(`${BASE_URL}/api/pages`, payload, { headers: writeHeaders(session) });
  createDuration.add(res.timings.duration);
  if (res.status !== 201) {
    unexpectedErrors.add(1);
  }
  check(res, { 'create ok': (r) => r.status === 201 });
}
