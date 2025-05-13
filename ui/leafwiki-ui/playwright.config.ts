import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  timeout: 10000,
  use: {
    baseURL: 'http://localhost:8080',
    headless: false,
    screenshot: 'only-on-failure',
  },
});
