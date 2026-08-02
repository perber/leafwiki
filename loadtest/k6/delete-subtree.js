// Scenario 2 (plans/loadtest-additional-scenarios.md) — recursive delete
// of a section subtree, at a light concurrency sweep (1 vs. N) and two
// subtree-size tiers, rather than a full VU sweep — this is primarily a
// data-shape stress test (does subtree size matter), not a concurrency
// one, per the plan.
//
// Delete is destructive and single-shot per target, so each VU deletes
// exactly one distinct, dedicated section (seeded via
// loadtest/seed/gen-nested, e.g. `--sections 22 --pages-per-section 10
// --start-index 1` for the small tier). Uses k6's per-vu-iterations
// executor (1 iteration/VU) rather than a duration-based loop.
//
// Run standalone, e.g.:
//   docker run --rm -i --network=host --user "$(id -u):$(id -g)" \
//     -v $(pwd)/loadtest/k6:/scripts -v $(pwd)/loadtest/seed:/seed \
//     -e BASE_URL=http://127.0.0.1:8091 -e SESSION_POOL_PATH=/seed/.session-pool.json \
//     -e SECTION_START=1 -e VUS=10 \
//     grafana/k6 run /scripts/delete-subtree.js
import http from 'k6/http';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { pickSession, writeHeaders } from './lib/auth.js';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8091';
const SESSION_POOL_PATH = __ENV.SESSION_POOL_PATH || '/seed/.session-pool.json';
const SECTION_START = parseInt(__ENV.SECTION_START || '1', 10);
const VUS = parseInt(__ENV.VUS || '1', 10);

const pool = JSON.parse(open(SESSION_POOL_PATH));

const deleteDuration = new Trend('delete_duration_ms', true);
const unexpectedErrors = new Counter('unexpected_errors');

export const options = {
  scenarios: {
    deleters: {
      executor: 'per-vu-iterations',
      vus: VUS,
      iterations: 1,
      maxDuration: '60s',
    },
  },
  thresholds: {
    unexpected_errors: ['count==0'],
  },
};

export default function () {
  const session = pickSession(pool, __VU);
  const sectionId = 'section-' + String(SECTION_START + __VU - 1).padStart(3, '0');

  const getRes = http.get(`${BASE_URL}/api/pages/${sectionId}`, { headers: writeHeaders(session) });
  if (getRes.status !== 200) {
    unexpectedErrors.add(1);
    return;
  }
  const version = getRes.json().version;

  const res = http.del(
    `${BASE_URL}/api/pages/${sectionId}?recursive=true&version=${encodeURIComponent(version)}`,
    null,
    { headers: writeHeaders(session) },
  );
  deleteDuration.add(res.timings.duration);
  if (res.status !== 200) {
    unexpectedErrors.add(1);
  }
  check(res, { 'delete ok': (r) => r.status === 200 });
}
