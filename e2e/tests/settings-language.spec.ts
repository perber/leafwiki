import test, { expect } from '@playwright/test';
import { getCsrfScript } from '../helpers/api';
import AccountSettingsPage from '../pages/AccountSettingsPage';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

// language is a per-user server setting (GET/PUT /api/user-settings), not
// per-browser localStorage — it persists across reloads AND leaks into every
// other spec file sharing this admin account. Always restore the default
// afterward, the same way settings-preferences.spec.ts resets autoSave.
test.afterEach(async ({ page }) => {
  await page
    .evaluate(async (csrfScript) => {
      const csrfToken = new Function(csrfScript)() as string;
      await fetch('/api/user-settings', {
        method: 'PUT',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
        body: JSON.stringify({ language: 'en' }),
      });
    }, getCsrfScript())
    .catch(() => {});
});

test.describe('Account settings — language', () => {
  test('switching the language updates the UI immediately and persists across reloads', async ({
    page,
  }) => {
    const loginPage = new LoginPage(page);
    const viewPage = new ViewPage(page);
    const settingsPage = new AccountSettingsPage(page);

    await loginPage.goto();
    await loginPage.login(user, password);
    await viewPage.expectUserLoggedIn();

    await settingsPage.goto();
    await expect(settingsPage.pageTitle()).toHaveText('Account');
    await expect(settingsPage.languageSelect()).toHaveText('English');

    await settingsPage.selectLanguage('Deutsch');

    // Applied immediately, without waiting for the PUT to resolve or a reload.
    await expect(settingsPage.pageTitle()).toHaveText('Konto');

    // Reload to prove this is backend-persisted, not just local UI state.
    await page.reload();
    await settingsPage.pageTitle().waitFor({ state: 'visible' });
    await expect(settingsPage.pageTitle()).toHaveText('Konto');
    await expect(settingsPage.languageSelect()).toHaveText('Deutsch');

    await settingsPage.selectLanguage('English');
    await expect(settingsPage.pageTitle()).toHaveText('Account');
  });
});
