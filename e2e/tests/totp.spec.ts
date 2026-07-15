import { expect, test } from '@playwright/test';
import LoginPage from '../pages/LoginPage';
import TotpDialog from '../pages/TotpDialog';
import ViewPage from '../pages/ViewPage';
import { generateTotpCode } from '../pages/totp';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

// These tests enable/disable TOTP on the shared admin account used by every
// other spec file, so they must run one at a time (never interleaved with
// each other) and each one must leave TOTP disabled again before finishing,
// or every other spec file's plain password login would start failing.
test.describe.configure({ mode: 'serial' });

test.describe('TOTP two-factor authentication', () => {
  // Tracks the secret of a TOTP setup started by the current test, so
  // afterEach can force-disable it even if the test fails partway through.
  let activeSecret: string | null = null;

  test.afterEach(async ({ page }) => {
    if (!activeSecret) return;

    const cookies = await page.context().cookies();
    const csrfToken = cookies.find(
      (c) => c.name === 'leafwiki_csrf' || c.name === '__Host-leafwiki_csrf',
    )?.value;

    if (csrfToken) {
      await page.request
        .post('/api/users/me/totp/disable', {
          headers: { 'X-CSRF-Token': csrfToken },
          data: { currentPassword: password, code: generateTotpCode(activeSecret) },
          failOnStatusCode: false,
        })
        .catch(() => {});
    }

    activeSecret = null;
  });

  // Runs through the full setup wizard (password -> QR/code -> recovery
  // codes) and returns the base32 secret so tests can generate valid codes.
  async function enableTotp(
    page: import('@playwright/test').Page,
    totpDialog: TotpDialog,
  ): Promise<{ secret: string; recoveryCodes: string[] }> {
    await totpDialog.openEnableDialog();
    await totpDialog.submitSetupPassword(password);

    const secret = await totpDialog.readManualKey();
    activeSecret = secret;

    await totpDialog.submitSetupCode(generateTotpCode(secret));
    const recoveryCodes = await totpDialog.readRecoveryCodes();
    expect(recoveryCodes.length).toBeGreaterThan(0);
    await totpDialog.finishSetup();

    return { secret, recoveryCodes };
  }

  test('enable TOTP, log out, and log back in with a TOTP code', async ({ page }) => {
    const loginPage = new LoginPage(page);
    const viewPage = new ViewPage(page);
    const totpDialog = new TotpDialog(page);

    await loginPage.goto();
    await loginPage.login(user, password);
    await viewPage.expectUserLoggedIn();

    const { secret } = await enableTotp(page, totpDialog);

    await viewPage.logout();
    expect(await viewPage.isLoggedOut()).toBe(true);

    // Password-only login must now stop at a TOTP prompt instead of logging in.
    await loginPage.login(user, password);
    const totpCodeInput = page.getByTestId('login-totp-code');
    await totpCodeInput.waitFor({ state: 'visible' });

    await totpCodeInput.fill(generateTotpCode(secret));
    await page.getByTestId('login-totp-submit').click();

    await viewPage.expectUserLoggedIn();
  });

  test('wrong TOTP code is rejected during login', async ({ page }) => {
    const loginPage = new LoginPage(page);
    const viewPage = new ViewPage(page);
    const totpDialog = new TotpDialog(page);

    await loginPage.goto();
    await loginPage.login(user, password);
    await viewPage.expectUserLoggedIn();

    const { secret } = await enableTotp(page, totpDialog);
    await viewPage.logout();

    await loginPage.login(user, password);
    const totpCodeInput = page.getByTestId('login-totp-code');
    await totpCodeInput.waitFor({ state: 'visible' });

    await totpCodeInput.fill('000000');
    await page.getByTestId('login-totp-submit').click();

    await expect(page.getByText('Invalid authentication code')).toBeVisible();
    // Must still be stuck on the TOTP step, not logged in.
    await expect(totpCodeInput).toBeVisible();

    // Recover the session with a valid code so afterEach's cleanup (which
    // needs an authenticated CSRF cookie) can run.
    await totpCodeInput.fill(generateTotpCode(secret));
    await page.getByTestId('login-totp-submit').click();
    await viewPage.expectUserLoggedIn();
  });

  test('user can log in with a recovery code after enabling TOTP', async ({ page }) => {
    const loginPage = new LoginPage(page);
    const viewPage = new ViewPage(page);
    const totpDialog = new TotpDialog(page);

    await loginPage.goto();
    await loginPage.login(user, password);
    await viewPage.expectUserLoggedIn();

    const { secret, recoveryCodes } = await enableTotp(page, totpDialog);
    await viewPage.logout();

    await loginPage.login(user, password);
    const totpCodeInput = page.getByTestId('login-totp-code');
    await totpCodeInput.waitFor({ state: 'visible' });

    await totpCodeInput.fill(recoveryCodes[0]);
    await page.getByTestId('login-totp-submit').click();
    await viewPage.expectUserLoggedIn();

    // The consumed recovery code must not work a second time.
    await viewPage.logout();
    await loginPage.login(user, password);
    await totpCodeInput.waitFor({ state: 'visible' });
    await totpCodeInput.fill(recoveryCodes[0]);
    await page.getByTestId('login-totp-submit').click();
    await expect(page.getByText('Invalid authentication code')).toBeVisible();

    // Recover with a fresh TOTP code so afterEach's cleanup can run.
    await totpCodeInput.fill(generateTotpCode(secret));
    await page.getByTestId('login-totp-submit').click();
    await viewPage.expectUserLoggedIn();
  });

  test('user can disable TOTP and log in normally again', async ({ page }) => {
    const loginPage = new LoginPage(page);
    const viewPage = new ViewPage(page);
    const totpDialog = new TotpDialog(page);

    await loginPage.goto();
    await loginPage.login(user, password);
    await viewPage.expectUserLoggedIn();

    const { secret } = await enableTotp(page, totpDialog);

    await totpDialog.openDisableDialog();
    await totpDialog.submitDisable(password, generateTotpCode(secret));

    // Once actually disabled, afterEach's extra disable-call is a harmless
    // no-op (rejected as "not enabled"); clear it so cleanup doesn't fire.
    activeSecret = null;

    await viewPage.logout();

    // Password-only login must work again, with no TOTP prompt.
    await loginPage.login(user, password);
    await viewPage.expectUserLoggedIn();
    await expect(page.getByTestId('login-totp-code')).toHaveCount(0);
  });
});
