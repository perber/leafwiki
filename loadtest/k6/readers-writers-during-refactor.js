// Follow-up to Scenario 1 / Scenario 4 — Patrick asked specifically: while
// a link-refactor (rename with rewriteLinks) touching many pages is in
// flight, can reads (page views + search queries) and writes (normal page
// saves) still perform well?
//
// Three concurrent k6 scenarios for the same STAGE_DURATION window:
//   - readers: constant-vus, alternates GET /api/search?q=... and
//     GET /api/pages/by-path (a normal page view) each iteration.
//   - writers: constant-vus, the same GET-then-PUT-own-page loop as
//     write-different-pages.js.
//   - refactorer: only added when REFACTOR_HUB_COUNT>0 — one VU doing
//     REFACTOR_HUB_COUNT sequential rename-with-refactor operations
//     (preview then apply, rewriteLinks:true) against high-in-degree hub
//     pages (seeded via loadtest/seed/gen-linked), one after another
//     (per-vu-iterations with a single VU sequences them rather than
//     firing all at once, so the "many affected pages" work is spread
//     across more of the measurement window instead of bunching at t=0).
//     REQUIRES --enable-link-refactor on the server.
//
// Run REFACTOR_HUB_COUNT=0 first for the baseline, then with it >0 for
// the comparison — same 0-vs-N methodology as every other reader/writer
// scenario this session.
//
// Run standalone, e.g.:
//   docker run --rm -i --network=host --user "$(id -u):$(id -g)" \
//     -v $(pwd)/loadtest/k6:/scripts -v $(pwd)/loadtest/seed:/seed \
//     -e BASE_URL=http://127.0.0.1:8091 -e SESSION_POOL_PATH=/seed/.session-pool.json \
//     -e READER_VUS=10 -e WRITER_VUS=5 -e REFACTOR_HUB_COUNT=3 -e REFACTOR_HUB_START=1 \
//     -e STAGE_DURATION=10s -e PAGE_COUNT=500 \
//     grafana/k6 run /scripts/readers-writers-during-refactor.js
import http from 'k6/http';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { pickSession, authHeaders, writeHeaders } from './lib/auth.js';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8091';
const PAGE_COUNT = parseInt(__ENV.PAGE_COUNT || '500', 10);
const SESSION_POOL_PATH = __ENV.SESSION_POOL_PATH || '/seed/.session-pool.json';
const READER_VUS = parseInt(__ENV.READER_VUS || '10', 10);
const WRITER_VUS = parseInt(__ENV.WRITER_VUS || '5', 10);
const REFACTOR_HUB_COUNT = parseInt(__ENV.REFACTOR_HUB_COUNT || '0', 10);
const REFACTOR_HUB_START = parseInt(__ENV.REFACTOR_HUB_START || '1', 10);
const STAGE_DURATION = __ENV.STAGE_DURATION || '10s';
const SEARCH_TERM = __ENV.SEARCH_TERM || 'Seed';

const pool = JSON.parse(open(SESSION_POOL_PATH));

const searchDuration = new Trend('search_duration_ms', true);
const readDuration = new Trend('read_by_path_duration_ms', true);
const saveDuration = new Trend('page_save_duration_ms', true);
const refactorApplyDuration = new Trend('refactor_apply_duration_ms', true);
const unexpectedErrors = new Counter('unexpected_errors');
const conflicts = new Counter('page_version_conflicts');

const scenarios = {
  readers: {
    executor: 'constant-vus',
    vus: READER_VUS,
    duration: STAGE_DURATION,
    exec: 'reader',
  },
  writers: {
    executor: 'constant-vus',
    vus: WRITER_VUS,
    duration: STAGE_DURATION,
    exec: 'writer',
  },
};

if (REFACTOR_HUB_COUNT > 0) {
  scenarios.refactorer = {
    executor: 'per-vu-iterations',
    vus: 1,
    iterations: REFACTOR_HUB_COUNT,
    maxDuration: STAGE_DURATION,
    exec: 'refactorer',
  };
}

export const options = {
  scenarios,
  thresholds: {
    unexpected_errors: ['count==0'],
  },
};

export function reader() {
  const session = pickSession(pool, __VU);
  const headers = authHeaders(session);

  const searchRes = http.get(`${BASE_URL}/api/search?q=${SEARCH_TERM}`, { headers, tags: { name: 'search' } });
  searchDuration.add(searchRes.timings.duration);
  check(searchRes, { 'search ok': (r) => r.status === 200 });

  const pageNum = Math.floor(Math.random() * PAGE_COUNT) + 1;
  const pageId = 'page-' + String(pageNum).padStart(5, '0');
  const readRes = http.get(`${BASE_URL}/api/pages/by-path?path=${pageId}`, { headers, tags: { name: 'by-path' } });
  readDuration.add(readRes.timings.duration);
  check(readRes, { 'read ok': (r) => r.status === 200 });
}

const vuState = {};

export function writer() {
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
    conflicts.add(1);
    const getRes = http.get(`${BASE_URL}/api/pages/${state.pageId}`, { headers: writeHeaders(session) });
    if (getRes.status === 200) {
      state.version = getRes.json().version;
    }
  } else {
    unexpectedErrors.add(1);
  }
}

export function refactorer() {
  // A dedicated admin-ish session for the refactor VU — reuse pool[0] for
  // simplicity, distinct from the reader/writer VUs' own sessions.
  const session = pickSession(pool, pool.length);
  const hubId = 'hub-' + String(REFACTOR_HUB_START + __ITER).padStart(3, '0');

  const getRes = http.get(`${BASE_URL}/api/pages/${hubId}`, { headers: writeHeaders(session) });
  if (getRes.status !== 200) {
    unexpectedErrors.add(1);
    return;
  }
  const page = getRes.json();
  const newTitle = page.title + ' Renamed';
  const newSlug = page.slug + '-renamed';

  const previewPayload = JSON.stringify({ kind: 'rename', title: newTitle, slug: newSlug });
  const previewRes = http.post(`${BASE_URL}/api/pages/${hubId}/refactor/preview`, previewPayload, {
    headers: writeHeaders(session),
  });
  if (previewRes.status !== 200) {
    unexpectedErrors.add(1);
    return;
  }

  const applyPayload = JSON.stringify({
    version: page.version,
    kind: 'rename',
    title: newTitle,
    slug: newSlug,
    rewriteLinks: true,
  });
  const applyRes = http.post(`${BASE_URL}/api/pages/${hubId}/refactor/apply`, applyPayload, {
    headers: writeHeaders(session),
  });
  refactorApplyDuration.add(applyRes.timings.duration);
  if (applyRes.status !== 200) {
    unexpectedErrors.add(1);
  }
  check(applyRes, { 'refactor apply ok': (r) => r.status === 200 });
}
