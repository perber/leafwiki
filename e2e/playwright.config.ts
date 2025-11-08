import { defineConfig } from '@playwright/test';

const baseURL = process.env.E2E_BASE_URL || 'http://localhost:8080';

export default defineConfig({
  testDir: './tests',
  timeout: 10000,
  use: {
    baseURL,
    headless: true,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
});
