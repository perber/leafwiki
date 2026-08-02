// Pin sweep — each VU owns one dedicated seeded page (page-{VU}) and
// repeatedly toggles its pinned flag. Mirrors write-different-pages.js's
// per-VU-owned-target/tracked-version pattern; no two VUs touch the same
// page, so this isolates PinPageUseCase cost from any real contention.
//
// Run standalone, e.g.:
//   docker run --rm -i --network=host --user "$(id -u):$(id -g)" \
//     -v $(pwd)/loadtest/k6:/scripts -v $(pwd)/loadtest/seed:/seed \
//     -e BASE_URL=http://127.0.0.1:8091 -e SESSION_POOL_PATH=/seed/.session-pool.json \
//     -e VUS=10 -e STAGE_DURATION=15s \
//     grafana/k6 run /scripts/pin-pages.js
import http from 'k6/http';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { pickSession, writeHeaders } from './lib/auth.js';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8091';
const PAGE_COUNT = parseInt(__ENV.PAGE_COUNT || '10000', 10);
const SESSION_POOL_PATH = __ENV.SESSION_POOL_PATH || '/seed/.session-pool.json';
const STAGE_DURATION = __ENV.STAGE_DURATION || '15s';

const pool = JSON.parse(open(SESSION_POOL_PATH));

const pinDuration = new Trend('pin_duration_ms', true);
const unexpectedErrors = new Counter('unexpected_errors');

const vuState = {};

export const options = {
  scenarios: {
    pinners: {
      executor: 'constant-vus',
      vus: parseInt(__ENV.VUS || '10', 10),
      duration: STAGE_DURATION,
    },
  },
  thresholds: {
    unexpected_errors: ['count==0'],
  },
};

export default function () {
  const session = pickSession(pool, __VU);
  let state = vuState[__VU];

  if (!state) {
    const pageNum = ((__VU - 1) % PAGE_COUNT) + 1;
    const pageId = 'page-' + String(pageNum).padStart(5, '0');
    const getRes = http.get(`${BASE_URL}/api/pages/${pageId}`, { headers: writeHeaders(session) });
    if (getRes.status !== 200) {
      unexpectedErrors.add(1);
      return;
    }
    const body = getRes.json();
    state = { pageId, version: body.version, pinned: !!body.pinned };
    vuState[__VU] = state;
  }

  state.pinned = !state.pinned;
  const payload = JSON.stringify({ version: state.version, pinned: state.pinned });

  const res = http.put(`${BASE_URL}/api/pages/${state.pageId}/pin`, payload, { headers: writeHeaders(session) });
  pinDuration.add(res.timings.duration);

  if (res.status === 200) {
    state.version = res.json().version;
    check(res, { 'pin ok': (r) => r.status === 200 });
  } else {
    unexpectedErrors.add(1);
  }
}
