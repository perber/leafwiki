// Scenario B — "many users editing the same page" conflict/throughput test.
//
// All VUs repeatedly GET-then-PUT one fixed page. This mirrors the editor's
// forceOverwrite() flow (docs/dev/processes.md, "Editor auto-save flow":
// re-fetch current version, then save) rather than blindly retrying a stale
// cached version, so it measures the realistic worst case: what happens
// when several people have the same page open and save around the same
// time. Expect a meaningful 409 page_version_conflict rate as VU count
// grows — that's the metric here, not just latency.
//
// Run via run.sh, or standalone, e.g.:
//   docker run --rm -i --network=host \
//     -v $(pwd)/loadtest/k6:/scripts -v $(pwd)/loadtest/seed:/seed \
//     -e BASE_URL=http://127.0.0.1:8091 -e SESSION_POOL_PATH=/seed/.session-pool.json \
//     -e TARGET_PAGE_ID=page-00001 \
//     grafana/k6 run /scripts/write-same-page.js --vus 10 --duration 20s
import http from 'k6/http';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { pickSession, writeHeaders } from './lib/auth.js';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8091';
const SESSION_POOL_PATH = __ENV.SESSION_POOL_PATH || '/seed/.session-pool.json';
const TARGET_PAGE_ID = __ENV.TARGET_PAGE_ID || 'page-00001';

const pool = JSON.parse(open(SESSION_POOL_PATH));

const conflicts = new Counter('page_version_conflicts');
const successes = new Counter('page_save_successes');
const unexpectedErrors = new Counter('unexpected_errors');
const saveDuration = new Trend('page_save_duration_ms', true);

export const options = {
  thresholds: {
    unexpected_errors: ['count==0'],
  },
};

export default function () {
  const session = pickSession(pool, __VU);

  const getRes = http.get(`${BASE_URL}/api/pages/${TARGET_PAGE_ID}`, { headers: writeHeaders(session) });
  if (getRes.status !== 200) {
    unexpectedErrors.add(1);
    return;
  }
  const body = getRes.json();

  const payload = JSON.stringify({
    version: body.version,
    title: body.title,
    slug: body.slug,
    content: `# ${body.title}\n\nEdited by VU ${__VU} at ${Date.now()}.\n`,
  });

  const res = http.put(`${BASE_URL}/api/pages/${TARGET_PAGE_ID}`, payload, { headers: writeHeaders(session) });
  saveDuration.add(res.timings.duration);

  if (res.status === 200) {
    successes.add(1);
    check(res, { 'save ok': (r) => r.status === 200 });
  } else if (res.status === 409) {
    conflicts.add(1);
  } else {
    unexpectedErrors.add(1);
  }
}
