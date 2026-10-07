import { expect, test } from '@playwright/test';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

// Queries a user can type (or reach mid-typing with search-as-you-type) that
// mix FTS5 operator characters with punctuation. None of them may fail.
const queries = [
  'foo-bar*',
  'report.v2*',
  'c#*',
  'a/b*',
  '(foo',
  'foo)',
  'foo AND (bar',
  '*foo',
  "it's* thing",
  'foo, bar',
  'http://example.com',
  '"unbalanced',
];

test('search API answers 200 for punctuation and operator edge cases', async ({ request }) => {
  const login = await request.post('/api/auth/login', {
    data: { identifier: user, password },
  });
  expect(login.status()).toBe(200);

  for (const q of queries) {
    const resp = await request.get('/api/search', { params: { q } });
    expect(resp.status(), `GET /api/search?q=${q}`).toBe(200);
  }
});
