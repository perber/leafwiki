import { Page } from '@playwright/test';
import { toAppPath } from './appPath';

// Drives the TOTP setup wizard (password -> QR/code -> recovery codes) and
// the TOTP disable flow, both now inline panels on the unified settings
// page instead of dialogs — opened by navigating to /settings/account.
// See ui/leafwiki-ui/src/features/settings/account/TotpPanel.tsx.
export default class TotpDialog {
  constructor(private page: Page) {}

  private async gotoAccountSettings() {
    await this.page.goto(toAppPath('/settings/account'));
  }

  async openEnableDialog() {
    await this.gotoAccountSettings();
    const passwordInput = this.page.getByTestId('totp-setup-password');
    await passwordInput.waitFor({ state: 'visible' });
  }

  async openDisableDialog() {
    await this.gotoAccountSettings();
    const passwordInput = this.page.getByTestId('totp-disable-password');
    await passwordInput.waitFor({ state: 'visible' });
  }

  // Step 1 of setup: confirm current password, advancing to the QR/code step.
  async submitSetupPassword(password: string) {
    const input = this.page.getByTestId('totp-setup-password');
    await input.waitFor({ state: 'visible' });
    await input.fill(password);
    await this.page.getByTestId('totp-setup-continue').click();
  }

  // Reads the manual-entry base32 secret shown on the QR/code step.
  async readManualKey(): Promise<string> {
    const el = this.page.getByTestId('totp-setup-manual-key');
    await el.waitFor({ state: 'visible' });
    const text = await el.textContent();
    if (!text) {
      throw new Error('TOTP setup manual key was empty');
    }
    return text.trim();
  }

  // Step 2 of setup: submit a TOTP code, advancing to the recovery-codes step.
  async submitSetupCode(code: string) {
    const input = this.page.getByTestId('totp-setup-code');
    await input.waitFor({ state: 'visible' });
    await input.fill(code);
    await this.page.getByTestId('totp-setup-enable').click();
  }

  // Reads the one-time recovery codes shown on the final setup step.
  async readRecoveryCodes(): Promise<string[]> {
    const el = this.page.getByTestId('totp-setup-recovery-codes');
    await el.waitFor({ state: 'visible' });
    const text = await el.textContent();
    if (!text) {
      throw new Error('TOTP recovery codes were empty');
    }
    return text
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean);
  }

  // Final step of setup: no explicit "done" action needed — the recovery
  // codes panel has no further confirm button, unlike the old dialog.
  async finishSetup() {
    await this.page.getByTestId('totp-setup-recovery-codes').waitFor({ state: 'visible' });
  }

  // Disable flow: password + a TOTP or recovery code in one step.
  async submitDisable(password: string, code: string) {
    await this.page.getByTestId('totp-disable-password').fill(password);
    await this.page.getByTestId('totp-disable-code').fill(code);
    await this.page.getByTestId('totp-disable-confirm').click();
  }
}
