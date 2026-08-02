// Scenario 1 (plans/loadtest-additional-scenarios.md) — readers-during-writers.
//
// Answers "can a visitor read while someone else is saving?" by running a
// `readers` scenario (constant-vus, swept via READER_VUS) and, when
// WRITER_VUS > 0, a concurrent `writers` scenario (constant-vus, fixed) in
// the same time window. When WRITER_VUS is 0, only `readers` runs — that's
// the isolated reader-only baseline the plan calls for, using the *same*
// reader code path as the with-writers run so the two are directly
// comparable.
//
// internal/core/tree/tree_service.go's global sync.RWMutex is only held for
// the brief on-disk-write portion of a save (~10ms observed); the expensive
// side effects (search/links/revision/tags/properties) run after the lock
// releases (internal/wiki/pages/update_page.go). Theory says readers should
// barely notice writers, since reads only block behind that short
// exclusive-lock window, not the whole save — this script measures whether
// that holds.
//
// Deliberately hits GET /api/pages/:id (plus, in 'content-links' mode,
// GET /api/pages/:id/links) — not GET /api/tree, which is already known to
// collapse at 10k pages regardless of writer activity (see the
// retrospective's "deliberately not done" section) and would swamp the
// specific signal being measured here.
//
// READER_MODE selects what a reader iteration does:
//   - 'content' (default): GET /api/pages/:id + GET /api/pages/by-path.
//     Both sit behind internal/core/tree/tree_service.go's RWMutex, held
//     only ~10ms per save — theory says this should barely degrade under
//     writers.
//   - 'content-links': GET /api/pages/:id + GET /api/pages/:id/links,
//     replicating what a real page view actually does — ui/leafwiki-ui's
//     PageViewer.tsx renders <BacklinkInfo /> on every successful view,
//     which auto-fires the /links call alongside content (unless the admin
//     set hideLinkMetadataSection, off by default). internal/links/links_store.go
//     guards every method — reads AND writes — with one process-wide
//     sync.Mutex (no WAL pragma, same gap class search/tags had before
//     their fixes) and holds it during the AddLinks/ReplaceLinksAndHeal
//     write that runs on every save. Theory says THIS reader mode should
//     degrade noticeably more under writers than 'content' does — run both
//     modes at the same VU levels and compare.
//
// Run via run-readers-during-writers.sh, or standalone, e.g.:
//   docker run --rm -i --network=host \
//     -v $(pwd)/loadtest/k6:/scripts -v $(pwd)/loadtest/seed:/seed \
//     -e BASE_URL=http://127.0.0.1:8091 -e SESSION_POOL_PATH=/seed/.session-pool.json \
//     -e READER_MODE=content-links -e READER_VUS=10 -e WRITER_VUS=10 -e STAGE_DURATION=20s \
//     grafana/k6 run /scripts/readers-during-writers.js
import http from 'k6/http';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { pickSession, authHeaders, writeHeaders } from './lib/auth.js';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8091';
const PAGE_COUNT = parseInt(__ENV.PAGE_COUNT || '10000', 10);
const SESSION_POOL_PATH = __ENV.SESSION_POOL_PATH || '/seed/.session-pool.json';
const READER_VUS = parseInt(__ENV.READER_VUS || '10', 10);
const WRITER_VUS = parseInt(__ENV.WRITER_VUS || '0', 10);
const STAGE_DURATION = __ENV.STAGE_DURATION || '20s';
const READER_MODE = __ENV.READER_MODE || 'content';

const pool = JSON.parse(open(SESSION_POOL_PATH));

const readerByIdDuration = new Trend('reader_get_by_id_duration_ms', true);
const readerByPathDuration = new Trend('reader_by_path_duration_ms', true);
const readerLinksDuration = new Trend('reader_links_duration_ms', true);
const conflicts = new Counter('page_version_conflicts');
const unexpectedErrors = new Counter('unexpected_errors');
const saveDuration = new Trend('page_save_duration_ms', true);

const scenarios = {
  readers: {
    executor: 'constant-vus',
    vus: READER_VUS,
    duration: STAGE_DURATION,
    exec: 'reader',
  },
};

if (WRITER_VUS > 0) {
  scenarios.writers = {
    executor: 'constant-vus',
    vus: WRITER_VUS,
    duration: STAGE_DURATION,
    exec: 'writer',
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

  const pageNum = Math.floor(Math.random() * PAGE_COUNT) + 1;
  const pageId = 'page-' + String(pageNum).padStart(5, '0');

  const byIdRes = http.get(`${BASE_URL}/api/pages/${pageId}`, { headers, tags: { name: 'get-by-id' } });
  readerByIdDuration.add(byIdRes.timings.duration);
  check(byIdRes, { 'get-by-id ok': (r) => r.status === 200 });

  if (READER_MODE === 'content-links') {
    const linksRes = http.get(`${BASE_URL}/api/pages/${pageId}/links`, { headers, tags: { name: 'links' } });
    readerLinksDuration.add(linksRes.timings.duration);
    check(linksRes, { 'links ok': (r) => r.status === 200 });
  } else {
    const byPathRes = http.get(`${BASE_URL}/api/pages/by-path?path=${pageId}`, {
      headers,
      tags: { name: 'by-path' },
    });
    readerByPathDuration.add(byPathRes.timings.duration);
    check(byPathRes, { 'by-path ok': (r) => r.status === 200 });
  }
}

// Same GET-then-PUT-own-page pattern as write-different-pages.js — each
// writer VU owns one distinct seeded page for the run's duration so this
// isolates the readers-vs-lock question from per-page write contention.
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
