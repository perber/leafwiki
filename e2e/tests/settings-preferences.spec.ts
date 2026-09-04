import test, { expect } from '@playwright/test';
import { createPage, getCsrfScript } from '../helpers/api';
import AccountSettingsPage from '../pages/AccountSettingsPage';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

// autoSave is now a per-user server setting (GET/PUT /api/user-settings), not
// per-browser localStorage — it persists across reloads AND leaks into every
// other spec file sharing this admin account. Always restore the default
// afterward, the same way totp.spec.ts resets TOTP in its afterEach.
test.afterEach(async ({ page }) => {
  await page
    .evaluate(async (csrfScript) => {
      const csrfToken = new Function(csrfScript)() as string;
      await fetch('/api/user-settings', {
        method: 'PUT',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
        body: JSON.stringify({ autoSave: true }),
      });
    }, getCsrfScript())
    .catch(() => {});
});

test.describe('Account settings — preferences', () => {
  test('toggling auto-save persists across reloads and matches the editor toolbar', async ({
    page,
  }) => {
    const loginPage = new LoginPage(page);
    const viewPage = new ViewPage(page);
    const settingsPage = new AccountSettingsPage(page);

    await loginPage.goto();
    await loginPage.login(user, password);
    await viewPage.expectUserLoggedIn();

    await settingsPage.goto();
    await expect(settingsPage.autoSaveCheckbox()).toBeChecked();

    await settingsPage.toggleAutoSave();
    await expect(settingsPage.autoSaveCheckbox()).not.toBeChecked();

    // Reload to prove this is backend-persisted, not just local UI state.
    await page.reload();
    await settingsPage.autoSaveCheckbox().waitFor({ state: 'visible' });
    await expect(settingsPage.autoSaveCheckbox()).not.toBeChecked();

    // Open a page in the editor and toggle auto-save back on from its toolbar
    // — Auto Save is the 3rd toolbar button (useToolbarActions.tsx), and
    // Toolbar.tsx only renders the first 2 directly (VISIBLE_BUTTONS = 2), so
    // it lives in the "More actions" overflow menu.
    await createPage(page, {
      title: 'Auto Save E2E Page',
      slug: 'auto-save-e2e-page',
      content: 'hello',
    });
    await viewPage.goto('/auto-save-e2e-page');
    await viewPage.clickEditPageButton();

    await page.getByTestId('toolbar-overflow-button').click();
    const autoSaveMenuItem = page.getByTestId('toggle-auto-save-menu-item');
    await autoSaveMenuItem.waitFor({ state: 'visible' });
    await autoSaveMenuItem.click();

    // Confirm Settings reflects the change made from the editor toolbar.
    await settingsPage.goto();
    await expect(settingsPage.autoSaveCheckbox()).toBeChecked();
  });
});
