// Scenario C — read-path baseline, same VU sweep as the write scenarios.
//
// GET /api/tree, GET /api/pages/by-path, GET /api/search only take the tree
// service's RLock (internal/core/tree/tree_service.go), so they're expected
// to scale much further than the write scenarios before latency degrades.
// Run at the same VU levels as write-different-pages.js / write-same-page.js
// so the two curves can be plotted side by side.
//
// Run via run.sh, or standalone, e.g.:
//   docker run --rm -i --network=host \
//     -v $(pwd)/loadtest/k6:/scripts -v $(pwd)/loadtest/seed:/seed \
//     -e BASE_URL=http://127.0.0.1:8091 -e SESSION_POOL_PATH=/seed/.session-pool.json \
//     grafana/k6 run /scripts/read-paths.js --vus 10 --duration 20s
import http from 'k6/http';
import { check } from 'k6';
import { pickSession, authHeaders } from './lib/auth.js';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8091';
const PAGE_COUNT = parseInt(__ENV.PAGE_COUNT || '10000', 10);
const SESSION_POOL_PATH = __ENV.SESSION_POOL_PATH || '/seed/.session-pool.json';

const pool = JSON.parse(open(SESSION_POOL_PATH));

export default function () {
  const session = pickSession(pool, __VU);
  const headers = authHeaders(session);

  const treeRes = http.get(`${BASE_URL}/api/tree`, { headers, tags: { name: 'tree' } });
  check(treeRes, { 'tree ok': (r) => r.status === 200 });

  const pageNum = Math.floor(Math.random() * PAGE_COUNT) + 1;
  const slug = 'page-' + String(pageNum).padStart(5, '0');
  const byPathRes = http.get(`${BASE_URL}/api/pages/by-path?path=${slug}`, {
    headers,
    tags: { name: 'by-path' },
  });
  check(byPathRes, { 'by-path ok': (r) => r.status === 200 });

  const searchRes = http.get(`${BASE_URL}/api/search?q=load`, { headers, tags: { name: 'search' } });
  check(searchRes, { 'search ok': (r) => r.status === 200 });
}
