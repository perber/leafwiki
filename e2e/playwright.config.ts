import { defineConfig } from '@playwright/test';

const baseURL = process.env.E2E_BASE_URL || 'http://localhost:8080';
const headless = process.env.E2E_HEADLESS !== 'false';

export default defineConfig({
  testDir: './tests',
  timeout: 1 * 60 * 1000, // 1 minute per test
  use: {
    baseURL,
    headless,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
});
