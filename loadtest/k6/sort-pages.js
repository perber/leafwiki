// Sort sweep — each VU owns one dedicated seeded section (section-{SECTION_START+VU-1})
// with several children and repeatedly reorders them (fetch current children
// once, then shuffle + PUT on every iteration — sort has no optimistic-
// concurrency version field, so no version tracking is needed). No two VUs
// touch the same section, isolating SortPagesUseCase cost from real
// contention.
//
// Needs seed data with sections that actually have children, e.g.:
//   go run ./loadtest/seed/gen-nested --data-dir <dir> --sections 10 --pages-per-section 20
//
// Run standalone, e.g.:
//   docker run --rm -i --network=host --user "$(id -u):$(id -g)" \
//     -v $(pwd)/loadtest/k6:/scripts -v $(pwd)/loadtest/seed:/seed \
//     -e BASE_URL=http://127.0.0.1:8091 -e SESSION_POOL_PATH=/seed/.session-pool.json \
//     -e SECTION_START=1 -e VUS=10 -e STAGE_DURATION=15s \
//     grafana/k6 run /scripts/sort-pages.js
import http from 'k6/http';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { pickSession, writeHeaders } from './lib/auth.js';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8091';
const SESSION_POOL_PATH = __ENV.SESSION_POOL_PATH || '/seed/.session-pool.json';
const SECTION_START = parseInt(__ENV.SECTION_START || '1', 10);
const STAGE_DURATION = __ENV.STAGE_DURATION || '15s';

const pool = JSON.parse(open(SESSION_POOL_PATH));

const sortDuration = new Trend('sort_duration_ms', true);
const unexpectedErrors = new Counter('unexpected_errors');

const vuState = {};

export const options = {
  scenarios: {
    sorters: {
      executor: 'constant-vus',
      vus: parseInt(__ENV.VUS || '10', 10),
      duration: STAGE_DURATION,
    },
  },
  thresholds: {
    unexpected_errors: ['count==0'],
  },
};

function shuffle(ids) {
  const arr = ids.slice();
  for (let i = arr.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [arr[i], arr[j]] = [arr[j], arr[i]];
  }
  return arr;
}

export default function () {
  const session = pickSession(pool, __VU);
  let state = vuState[__VU];

  if (!state) {
    const sectionId = 'section-' + String(SECTION_START + __VU - 1).padStart(3, '0');
    const getRes = http.get(`${BASE_URL}/api/pages/${sectionId}`, { headers: writeHeaders(session) });
    if (getRes.status !== 200) {
      unexpectedErrors.add(1);
      return;
    }
    const children = getRes.json().children || [];
    if (children.length < 2) {
      unexpectedErrors.add(1);
      return;
    }
    state = { sectionId, childIDs: children.map((c) => c.id) };
    vuState[__VU] = state;
  }

  const orderedIds = shuffle(state.childIDs);
  const res = http.put(
    `${BASE_URL}/api/pages/${state.sectionId}/sort`,
    JSON.stringify({ orderedIds }),
    { headers: writeHeaders(session) },
  );
  sortDuration.add(res.timings.duration);
  if (res.status !== 200) {
    unexpectedErrors.add(1);
  }
  check(res, { 'sort ok': (r) => r.status === 200 });
}
