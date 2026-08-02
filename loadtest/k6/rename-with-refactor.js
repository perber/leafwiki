// Scenario 4 (plans/loadtest-additional-scenarios.md) — rename with link
// rewrite, at varying hub in-degree (data-shape sweep) rather than a VU
// sweep. Confirmed by reading internal/wiki/pages/refactor.go:
// ApplyPageRefactorUseCase.Execute internally calls UpdatePageUseCase
// itself for the renamed page, so the client-side flow is exactly two
// calls — POST .../refactor/preview then POST .../refactor/apply with
// rewriteLinks:true — no separate third PUT needed for a pure rename.
//
// REQUIRES the server to be started with --enable-link-refactor (default
// false, confirmed in cmd/leafwiki/main.go) or these endpoints 404.
//
// Each VU renames exactly one distinct, dedicated hub page (seeded via
// loadtest/seed/gen-linked, e.g. `--hubs 10 --linkers-per-hub 10
// --hub-start-index 1` for a low-in-degree tier, a separate invocation
// with a higher --linkers-per-hub and --hub-start-index for a high tier)
// — renaming is a one-shot identity change per hub, so this is a
// per-vu-iterations scenario, not a duration loop.
//
// Run standalone, e.g.:
//   docker run --rm -i --network=host --user "$(id -u):$(id -g)" \
//     -v $(pwd)/loadtest/k6:/scripts -v $(pwd)/loadtest/seed:/seed \
//     -e BASE_URL=http://127.0.0.1:8091 -e SESSION_POOL_PATH=/seed/.session-pool.json \
//     -e HUB_START=1 -e VUS=5 \
//     grafana/k6 run /scripts/rename-with-refactor.js
import http from 'k6/http';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { pickSession, writeHeaders } from './lib/auth.js';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8091';
const SESSION_POOL_PATH = __ENV.SESSION_POOL_PATH || '/seed/.session-pool.json';
const HUB_START = parseInt(__ENV.HUB_START || '1', 10);
const VUS = parseInt(__ENV.VUS || '1', 10);

const pool = JSON.parse(open(SESSION_POOL_PATH));

const previewDuration = new Trend('refactor_preview_duration_ms', true);
const applyDuration = new Trend('refactor_apply_duration_ms', true);
const affectedPages = new Trend('refactor_affected_pages', false);
const matchedLinks = new Trend('refactor_matched_links', false);
const unexpectedErrors = new Counter('unexpected_errors');

export const options = {
  scenarios: {
    renamers: {
      executor: 'per-vu-iterations',
      vus: VUS,
      iterations: 1,
      maxDuration: '120s',
    },
  },
  thresholds: {
    unexpected_errors: ['count==0'],
  },
};

export default function () {
  const session = pickSession(pool, __VU);
  const hubId = 'hub-' + String(HUB_START + __VU - 1).padStart(3, '0');

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
  previewDuration.add(previewRes.timings.duration);
  if (previewRes.status !== 200) {
    unexpectedErrors.add(1);
    return;
  }
  const preview = previewRes.json();
  affectedPages.add(preview.counts ? preview.counts.affectedPages : 0);
  matchedLinks.add(preview.counts ? preview.counts.matchedLinks : 0);

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
  applyDuration.add(applyRes.timings.duration);
  if (applyRes.status !== 200) {
    unexpectedErrors.add(1);
  }
  check(applyRes, { 'apply ok': (r) => r.status === 200 });
}
