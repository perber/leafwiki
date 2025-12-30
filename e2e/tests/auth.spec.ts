import test from '@playwright/test';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

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
