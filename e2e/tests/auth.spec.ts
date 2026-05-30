import { expect, test } from '@playwright/test';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';
import { toAppPath } from '../pages/appPath';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

function oauthAuthorizeReturnTo(): string {
  const base = new URL(process.env.E2E_BASE_URL || 'http://localhost:8080');
  const url = new URL(toAppPath('/oauth/authorize'), base);
  url.searchParams.set('client_id', 'leafwiki-local-mcp');
  url.searchParams.set('response_type', 'code');
  url.searchParams.set('redirect_uri', 'http://127.0.0.1:49152/callback');
  url.searchParams.set('scope', 'leafwiki:mcp');
  url.searchParams.set('state', 'login-return-to');
  url.searchParams.set('code_challenge', 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQ');
  url.searchParams.set('code_challenge_method', 'S256');
  return url.toString();
}

function expectAuthorizeURL(url: URL) {
  expect(url.origin).toBe(new URL(process.env.E2E_BASE_URL || 'http://localhost:8080').origin);
  expect(url.pathname).toBe(toAppPath('/oauth/authorize'));
  expect(url.searchParams.get('client_id')).toBe('leafwiki-local-mcp');
  expect(url.searchParams.get('state')).toBe('login-return-to');
}

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

test('login without returnTo leaves oauth authorize flow unset', async ({ page }) => {
  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login(user, password);

  await expect(page).not.toHaveURL(/\/login/);
  expect(new URL(page.url()).pathname).not.toBe(toAppPath('/oauth/authorize'));
});

test('login uses safe oauth returnTo for full-page navigation', async ({ page }) => {
  const returnTo = oauthAuthorizeReturnTo();
  await page.goto(toAppPath(`/login?returnTo=${encodeURIComponent(returnTo)}`));

  const loginPage = new LoginPage(page);
  await loginPage.login(user, password);

  await page.waitForURL((url) => url.pathname === toAppPath('/oauth/authorize'));
  expectAuthorizeURL(new URL(page.url()));
});

test('login ignores unsafe external returnTo', async ({ page }) => {
  await page.goto(
    toAppPath(`/login?returnTo=${encodeURIComponent('https://example.com/oauth/authorize')}`),
  );

  const loginPage = new LoginPage(page);
  await loginPage.login(user, password);

  await expect(page).not.toHaveURL(/example\.com/);
  await expect(page).not.toHaveURL(/\/login/);
  expect(new URL(page.url()).pathname).not.toBe(toAppPath('/oauth/authorize'));
});

test('already-authenticated login page uses safe oauth returnTo', async ({ page }) => {
  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login(user, password);
  await expect(page).not.toHaveURL(/\/login/);

  const returnTo = oauthAuthorizeReturnTo();
  await page.goto(toAppPath(`/login?returnTo=${encodeURIComponent(returnTo)}`));

  await page.waitForURL((url) => url.pathname === toAppPath('/oauth/authorize'));
  expectAuthorizeURL(new URL(page.url()));
});
