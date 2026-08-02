// Copy sweep — each VU repeatedly copies the same fixed source page into a
// brand-new, uniquely-slugged root-level target. Like create.js, every
// iteration produces a new page so there's no cross-iteration conflict;
// this isolates CopyPageUseCase (read source + write copy) under
// concurrency.
//
// Run standalone, e.g.:
//   docker run --rm -i --network=host --user "$(id -u):$(id -g)" \
//     -v $(pwd)/loadtest/k6:/scripts -v $(pwd)/loadtest/seed:/seed \
//     -e BASE_URL=http://127.0.0.1:8091 -e SESSION_POOL_PATH=/seed/.session-pool.json \
//     -e SOURCE_ID=page-00001 -e VUS=10 -e STAGE_DURATION=15s \
//     grafana/k6 run /scripts/copy-pages.js
import http from 'k6/http';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { pickSession, writeHeaders } from './lib/auth.js';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8091';
const SESSION_POOL_PATH = __ENV.SESSION_POOL_PATH || '/seed/.session-pool.json';
const SOURCE_ID = __ENV.SOURCE_ID || 'page-00001';
const VUS = parseInt(__ENV.VUS || '10', 10);
const STAGE_DURATION = __ENV.STAGE_DURATION || '15s';
const RUN_ID = __ENV.RUN_ID || String(Date.now());

const pool = JSON.parse(open(SESSION_POOL_PATH));

const copyDuration = new Trend('copy_duration_ms', true);
const unexpectedErrors = new Counter('unexpected_errors');

export const options = {
  scenarios: {
    copiers: {
      executor: 'constant-vus',
      vus: VUS,
      duration: STAGE_DURATION,
    },
  },
  thresholds: {
    unexpected_errors: ['count==0'],
  },
};

export default function () {
  const session = pickSession(pool, __VU);
  const slug = `load-copy-${RUN_ID}-vu${__VU}-${__ITER}`;

  const payload = JSON.stringify({
    targetParentId: null,
    title: `Load Copy ${RUN_ID} VU${__VU}/${__ITER}`,
    slug,
  });

  const res = http.post(`${BASE_URL}/api/pages/copy/${SOURCE_ID}`, payload, { headers: writeHeaders(session) });
  copyDuration.add(res.timings.duration);
  if (res.status !== 201) {
    unexpectedErrors.add(1);
  }
  check(res, { 'copy ok': (r) => r.status === 201 });
}
