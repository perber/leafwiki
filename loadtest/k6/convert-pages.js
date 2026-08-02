// Convert sweep — each VU owns one dedicated seeded page (page-{VU}) and
// repeatedly flips it page <-> section. handleConvert returns 204 (no
// body), so unlike pin/write-different-pages the version can't be carried
// forward from the response — each iteration re-GETs the page first. That
// makes this scenario's cost = one read + one convert, by necessity, not
// by choice.
//
// Run standalone, e.g.:
//   docker run --rm -i --network=host --user "$(id -u):$(id -g)" \
//     -v $(pwd)/loadtest/k6:/scripts -v $(pwd)/loadtest/seed:/seed \
//     -e BASE_URL=http://127.0.0.1:8091 -e SESSION_POOL_PATH=/seed/.session-pool.json \
//     -e VUS=10 -e STAGE_DURATION=15s \
//     grafana/k6 run /scripts/convert-pages.js
import http from 'k6/http';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { pickSession, writeHeaders } from './lib/auth.js';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8091';
const PAGE_COUNT = parseInt(__ENV.PAGE_COUNT || '10000', 10);
const SESSION_POOL_PATH = __ENV.SESSION_POOL_PATH || '/seed/.session-pool.json';
const STAGE_DURATION = __ENV.STAGE_DURATION || '15s';

const pool = JSON.parse(open(SESSION_POOL_PATH));

const convertDuration = new Trend('convert_duration_ms', true);
const unexpectedErrors = new Counter('unexpected_errors');

const vuState = {};

export const options = {
  scenarios: {
    converters: {
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
    state = { pageId: 'page-' + String(pageNum).padStart(5, '0'), kind: 'page' };
    vuState[__VU] = state;
  }

  const getRes = http.get(`${BASE_URL}/api/pages/${state.pageId}`, { headers: writeHeaders(session) });
  if (getRes.status !== 200) {
    unexpectedErrors.add(1);
    return;
  }
  const version = getRes.json().version;
  const targetKind = state.kind === 'page' ? 'section' : 'page';

  const payload = JSON.stringify({ version, targetKind });
  const res = http.post(`${BASE_URL}/api/pages/convert/${state.pageId}`, payload, { headers: writeHeaders(session) });
  convertDuration.add(res.timings.duration);

  if (res.status === 204) {
    state.kind = targetKind;
    check(res, { 'convert ok': (r) => r.status === 204 });
  } else {
    unexpectedErrors.add(1);
  }
}
