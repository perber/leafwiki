import { expect, test } from '@playwright/test';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

// GET /api/auth/me must never return 401, even without a session.
// A 401 causes browsers behind a Basic Auth reverse proxy (e.g. Traefik
// basicAuth middleware) to discard their cached credentials and re-prompt.
test('GET /api/auth/me without session returns 200 with null body, not 401', async ({
  request,
}) => {
  const resp = await request.get('/api/auth/me');
  expect(resp.status()).toBe(200);
  const body = await resp.json();
  expect(body).toBeNull();
});

test('GET /api/auth/me with valid session returns the authenticated user', async ({ request }) => {
  const login = await request.post('/api/auth/login', {
    data: { identifier: user, password },
  });
  expect(login.status()).toBe(200);

  const resp = await request.get('/api/auth/me');
  expect(resp.status()).toBe(200);
  const body = await resp.json();
  expect(body).not.toBeNull();
  expect(body.username).toBe(user);
  expect(body.role).toBe('admin');
});

test('GET /api/auth/me has Cache-Control: no-store on unauthenticated requests', async ({
  request,
}) => {
  const resp = await request.get('/api/auth/me');
  expect(resp.headers()['cache-control']).toBe('no-store');
});

test('GET /api/auth/me has Cache-Control: no-store on authenticated requests', async ({
  request,
}) => {
  await request.post('/api/auth/login', {
    data: { identifier: user, password },
  });
  const resp = await request.get('/api/auth/me');
  expect(resp.headers()['cache-control']).toBe('no-store');
});

test('failed login', async ({ page }) => {
  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login(user, 'failed');
  await loginPage.expectInvalidCredentialsError();
});

test('config error shows allow-insecure message instead of object text', async ({ page }) => {
  await page.route('**/api/config', async (route) => {
    await route.fulfill({
      status: 400,
      contentType: 'application/json',
      body: JSON.stringify({
        error: {
          code: 'auth_cookie_failed',
          message:
            'HTTPS is required for auth cookies. Use HTTPS or start LeafWiki with --allow-insecure for trusted plain HTTP setups.',
          template: 'https required for auth cookies use allow insecure',
        },
      }),
    });
  });

  const loginPage = new LoginPage(page);
  await loginPage.goto();

  await expect(page.getByText(/--allow-insecure/, { exact: false })).toBeVisible();
  await expect(
    page.getByText(
      'HTTPS is required for auth cookies. Use HTTPS or start LeafWiki with --allow-insecure for trusted plain HTTP setups.',
    ),
  ).toBeVisible();
  await expect(page.getByText('[object Object]')).toHaveCount(0);
});

// logout test
test('logout', async ({ page }) => {
  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login(user, password);
  const viewPage = new ViewPage(page);
  await viewPage.expectUserLoggedIn();
  await viewPage.logout();
  const loggedOut = await viewPage.isLoggedOut();
  test.expect(loggedOut).toBe(true);
});
