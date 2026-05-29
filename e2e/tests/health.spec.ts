import { expect, test } from '@playwright/test';

test('GET /api/health returns 200 with valid check fields', async ({ request }) => {
  const resp = await request.get('/api/health');
  expect(resp.status()).toBe(200);

  const body = (await resp.json()) as { status: string; checks: Record<string, string> };
  expect(body.status).toBe('ok');
  expect(body.checks.sqlite).toBe('ok');
  expect(body.checks.data_dir).toBe('ok');
  // search may still be indexing on a fresh server
  expect(['ok', 'indexing']).toContain(body.checks.search);
});

test('GET /api/health does not require authentication', async ({ request }) => {
  const resp = await request.get('/api/health');
  // Must never redirect to login or return 401/403
  expect([200, 503]).toContain(resp.status());
});
