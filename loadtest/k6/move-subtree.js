// Scenario 3 (plans/loadtest-additional-scenarios.md) — cross-parent move
// of a section subtree, at a light concurrency sweep (1 vs. N) and two
// subtree-size tiers. Same one-shot-per-target shape as delete-subtree.js:
// each VU moves exactly one distinct, dedicated section into a shared
// destination parent (MOVE_TARGET_ID — a small section that must already
// exist, e.g. gen-nested `--sections 1 --pages-per-section 1
// --start-index 900`), which gives every move a real path change (source
// sections must NOT already be children of the destination) and so
// exercises rewritePathChangedSubtree's relative-link rewriting inside the
// moved subtree, not a same-parent no-op.
//
// Run standalone, e.g.:
//   docker run --rm -i --network=host --user "$(id -u):$(id -g)" \
//     -v $(pwd)/loadtest/k6:/scripts -v $(pwd)/loadtest/seed:/seed \
//     -e BASE_URL=http://127.0.0.1:8091 -e SESSION_POOL_PATH=/seed/.session-pool.json \
//     -e SECTION_START=12 -e VUS=10 -e MOVE_TARGET_ID=section-900 \
//     grafana/k6 run /scripts/move-subtree.js
import http from 'k6/http';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { pickSession, writeHeaders } from './lib/auth.js';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8091';
const SESSION_POOL_PATH = __ENV.SESSION_POOL_PATH || '/seed/.session-pool.json';
const SECTION_START = parseInt(__ENV.SECTION_START || '1', 10);
const VUS = parseInt(__ENV.VUS || '1', 10);
const MOVE_TARGET_ID = __ENV.MOVE_TARGET_ID || 'section-900';

const pool = JSON.parse(open(SESSION_POOL_PATH));

const moveDuration = new Trend('move_duration_ms', true);
const unexpectedErrors = new Counter('unexpected_errors');

export const options = {
  scenarios: {
    movers: {
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

  const payload = JSON.stringify({ version, parentId: MOVE_TARGET_ID });
  const res = http.put(`${BASE_URL}/api/pages/${sectionId}/move`, payload, {
    headers: writeHeaders(session),
  });
  moveDuration.add(res.timings.duration);
  if (res.status !== 200) {
    unexpectedErrors.add(1);
  }
  check(res, { 'move ok': (r) => r.status === 200 });
}
