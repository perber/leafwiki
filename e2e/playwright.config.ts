import { defineConfig } from '@playwright/test';

// run.sh (local mode) starts one server per worker and lists their ports in
// E2E_PORTS; each worker process talks only to its own, so tests never race on
// shared pages, users or settings. Set before helpers read E2E_BASE_URL.
const ports = process.env.E2E_PORTS?.split(',');
const workerIndex = process.env.TEST_PARALLEL_INDEX;
if (ports?.[0] && workerIndex !== undefined) {
  process.env.E2E_BASE_URL = `http://localhost:${ports[Number(workerIndex)]}`;
}

const baseURL = process.env.E2E_BASE_URL || 'http://localhost:8080';
const headless = process.env.E2E_HEADLESS !== 'false';
const maxFailures = Number(process.env.E2E_MAX_FAILURES || '1');

export default defineConfig({
  testDir: './tests',
  timeout: 3 * 60 * 1000, // 3 minutes per test
  maxFailures,
  // Spreads the tests of one file (page.spec.ts holds ~40% of them) over workers.
  fullyParallel: true,
  use: {
    baseURL,
    headless,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    // Video records every test (dropped on pass) and costs CPU; CI keeps it,
    // locally the trace of a failure is enough.
    video: process.env.CI ? 'retain-on-failure' : 'off',
  },
});
