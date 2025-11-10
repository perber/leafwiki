import test from '@playwright/test';
import LoginPage from '../pages/LoginPage';

const user = process.env.E2E_ADMIN_USER || 'admin';

test('failed login', async ({ page }) => {
  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login(user, 'failed');
  await loginPage.expectInvalidCredentialsError();
});

// logout test
