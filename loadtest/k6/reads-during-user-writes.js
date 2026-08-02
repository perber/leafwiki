// Load test for internal/core/auth/user_store.go — measure only, no fix
// applied yet (see the plan for why: users.db is source-of-truth account
// data, not a derived/rebuildable index like search/tags/links, and it's
// special-cased by three separate backup/restore mechanisms, so a WAL
// change there needs its own explicit sign-off, not a reflexive copy of
// the search/tags fix).
//
// RequireAuth -> SessionManager.ValidateToken -> UserStore.GetUserByID
// (internal/http/middleware/auth/auth.go, internal/core/auth/session_manager.go)
// runs on EVERY authenticated request in the app. UserStore.Connect() only
// mutexes the lazy-open check, not the query methods themselves (unlike
// search/tags/links, which serialize every method behind one sync.Mutex) —
// so this store shouldn't show the "everything queues behind one lock"
// symptom. The question is narrower: in the current rollback-journal mode
// (busy_timeout(5000) set, no WAL), does a concurrent users.db WRITE (role
// change, password change, TOTP ops) still stall concurrent READS (i.e.
// every other authenticated request in the system) via SQLite's own
// file-level locking during commit?
//
// readers: constant-vus, swept, hitting GET /api/pages/by-path — a plain
// authenticated read that exercises RequireAuth/GetUserByID without
// touching anything else interesting (TreeService's own lock is already
// well-characterized in the companion readers-during-writers.js results).
//
// writers: a small FIXED vu count doing admin PUT /api/users/:id role
// toggles against dedicated loadtest-writer-* accounts (NOT the reader
// pool). Deliberately omits the "password" field so UserService.UpdateUser
// skips bcrypt entirely (internal/core/auth/user_service.go: `if password
// != "" { bcrypt... }`) -- confirmed by reading the code. That keeps this
// scenario's write RATE bounded only by DB/lock cost, not by bcrypt's
// ~650ms/op (observed elsewhere this session), which would otherwise mask
// the very thing being measured.
//
// Run via run-reads-during-user-writes.sh, or standalone, e.g.:
//   docker run --rm -i --network=host \
//     -v $(pwd)/loadtest/k6:/scripts -v $(pwd)/loadtest/seed:/seed \
//     -e BASE_URL=http://127.0.0.1:8091 -e SESSION_POOL_PATH=/seed/.session-pool.json \
//     -e ADMIN_SESSION_PATH=/seed/.admin-session.json -e WRITER_ACCOUNTS_PATH=/seed/.writer-accounts.json \
//     -e READER_VUS=10 -e WRITER_VUS=3 -e STAGE_DURATION=20s \
//     grafana/k6 run /scripts/reads-during-user-writes.js
import http from 'k6/http';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { pickSession, authHeaders, writeHeaders } from './lib/auth.js';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8091';
const PAGE_COUNT = parseInt(__ENV.PAGE_COUNT || '10000', 10);
const SESSION_POOL_PATH = __ENV.SESSION_POOL_PATH || '/seed/.session-pool.json';
const ADMIN_SESSION_PATH = __ENV.ADMIN_SESSION_PATH || '/seed/.admin-session.json';
const WRITER_ACCOUNTS_PATH = __ENV.WRITER_ACCOUNTS_PATH || '/seed/.writer-accounts.json';
const READER_VUS = parseInt(__ENV.READER_VUS || '10', 10);
const WRITER_VUS = parseInt(__ENV.WRITER_VUS || '0', 10);
const STAGE_DURATION = __ENV.STAGE_DURATION || '20s';
const READER_TARGET_PATH = __ENV.READER_TARGET_PATH || 'page-00001';

const pool = JSON.parse(open(SESSION_POOL_PATH));
const adminSession = WRITER_VUS > 0 ? JSON.parse(open(ADMIN_SESSION_PATH)) : null;
const writerAccounts = WRITER_VUS > 0 ? JSON.parse(open(WRITER_ACCOUNTS_PATH)) : [];

const readerDuration = new Trend('reader_by_path_duration_ms', true);
const writerDuration = new Trend('user_update_duration_ms', true);
const unexpectedErrors = new Counter('unexpected_errors');

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

  const res = http.get(`${BASE_URL}/api/pages/by-path?path=${READER_TARGET_PATH}`, {
    headers,
    tags: { name: 'by-path' },
  });
  readerDuration.add(res.timings.duration);
  check(res, { 'by-path ok': (r) => r.status === 200 });
}

export function writer() {
  const account = writerAccounts[(__VU - 1) % writerAccounts.length];
  const role = Math.random() < 0.5 ? 'editor' : 'viewer';

  // No "password" field -> UserService.UpdateUser skips bcrypt and keeps
  // the existing hash (internal/core/auth/user_service.go), so this write
  // is fast and DB/lock cost isn't masked by hashing latency.
  const payload = JSON.stringify({
    username: account.username,
    email: account.email,
    role,
  });

  const res = http.put(`${BASE_URL}/api/users/${account.id}`, payload, {
    headers: writeHeaders(adminSession),
  });
  writerDuration.add(res.timings.duration);

  if (res.status !== 200) {
    unexpectedErrors.add(1);
  }
  check(res, { 'user update ok': (r) => r.status === 200 });
}
