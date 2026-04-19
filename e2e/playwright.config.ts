import { defineConfig } from '@playwright/test';

const baseURL = process.env.E2E_BASE_URL || 'http://localhost:8080';
const headless = process.env.E2E_HEADLESS !== 'false';
const maxFailures = Number(process.env.E2E_MAX_FAILURES || '1');

export default defineConfig({
  testDir: './tests',
  timeout: 3 * 60 * 1000, // 3 minutes per test
  maxFailures,
  use: {
    baseURL,
    headless,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
});
