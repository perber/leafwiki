// Scenario A — "many users editing different pages" concurrency sweep.
//
// Each VU owns one distinct seeded page (page-{VU mod PAGE_COUNT}) for the
// whole run and repeatedly saves it. Since no two VUs ever touch the same
// page, this isolates the effect of the global tree-service lock
// (internal/core/tree/tree_service.go) from any real per-page contention:
// if throughput doesn't scale with VU count despite zero page overlap, the
// lock (not page contention) is the ceiling.
//
// Run via run.sh, or standalone, e.g.:
//   docker run --rm -i --network=host \
//     -v $(pwd)/loadtest/k6:/scripts -v $(pwd)/loadtest/seed:/seed \
//     -e BASE_URL=http://127.0.0.1:8091 -e SESSION_POOL_PATH=/seed/.session-pool.json \
//     grafana/k6 run /scripts/write-different-pages.js --vus 10 --duration 20s
import http from 'k6/http';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { pickSession, writeHeaders } from './lib/auth.js';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8091';
const PAGE_COUNT = parseInt(__ENV.PAGE_COUNT || '10000', 10);
const SESSION_POOL_PATH = __ENV.SESSION_POOL_PATH || '/seed/.session-pool.json';

const pool = JSON.parse(open(SESSION_POOL_PATH));

const conflicts = new Counter('page_version_conflicts');
const unexpectedErrors = new Counter('unexpected_errors');
const saveDuration = new Trend('page_save_duration_ms', true);

// Per-VU state persists across iterations within a VU's lifetime (k6 keeps
// the same JS VM per VU for the default executor), so we fetch each VU's
// starting version once and then track it locally from PUT responses.
const vuState = {};

export const options = {
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
    state = { pageId, version: body.version, title: body.title, slug: body.slug, n: 0 };
    vuState[__VU] = state;
  }

  state.n += 1;
  const payload = JSON.stringify({
    version: state.version,
    title: state.title,
    slug: state.slug,
    content: `# ${state.title}\n\nEdited by VU ${__VU}, iteration ${state.n}, at ${Date.now()}.\n`,
  });

  const res = http.put(`${BASE_URL}/api/pages/${state.pageId}`, payload, { headers: writeHeaders(session) });
  saveDuration.add(res.timings.duration);

  if (res.status === 200) {
    state.version = res.json().version;
    check(res, { 'save ok': (r) => r.status === 200 });
  } else if (res.status === 409) {
    // Unexpected here (this VU is the only writer of its page) — resync and
    // keep going rather than spinning on a stale version.
    conflicts.add(1);
    const getRes = http.get(`${BASE_URL}/api/pages/${state.pageId}`, { headers: writeHeaders(session) });
    if (getRes.status === 200) {
      state.version = getRes.json().version;
    }
  } else {
    unexpectedErrors.add(1);
  }
}
