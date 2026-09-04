import test, { expect } from '@playwright/test';
import { getCsrfScript } from '../helpers/api';
import AccountSettingsPage from '../pages/AccountSettingsPage';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

// dateFormat / timeFormat are per-user server settings (GET/PUT
// /api/user-settings), shared with every other spec on this admin account —
// always restore the "locale" defaults afterward.
test.afterEach(async ({ page }) => {
  await page
    .evaluate(async (csrfScript) => {
      const csrfToken = new Function(csrfScript)() as string;
      await fetch('/api/user-settings', {
        method: 'PUT',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
        body: JSON.stringify({ dateFormat: 'locale', timeFormat: 'locale' }),
      });
    }, getCsrfScript())
    .catch(() => {});
});

test.describe('Account settings — date & time format', () => {
  test('picking a date/time format persists across reloads', async ({ page }) => {
    const loginPage = new LoginPage(page);
    const viewPage = new ViewPage(page);
    const settingsPage = new AccountSettingsPage(page);

    await loginPage.goto();
    await loginPage.login(user, password);
    await viewPage.expectUserLoggedIn();

    await settingsPage.goto();
    await expect(settingsPage.dateFormatSelect()).toHaveText('Follow language');
    await expect(settingsPage.timeFormatSelect()).toHaveText('Follow language');

    await settingsPage.selectDateFormat('27.08.2026');
    await settingsPage.selectTimeFormat('24-hour (14:30)');

    await page.reload();
    await settingsPage.dateFormatSelect().waitFor({ state: 'visible' });
    await expect(settingsPage.dateFormatSelect()).toHaveText('27.08.2026');
    await expect(settingsPage.timeFormatSelect()).toHaveText('24-hour (14:30)');
  });
});
