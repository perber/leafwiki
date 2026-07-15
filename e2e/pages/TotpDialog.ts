import { Page } from '@playwright/test';

// Drives the TOTP setup wizard (password -> QR/code -> recovery codes) and
// the TOTP disable dialog, both opened from the user avatar dropdown menu.
// See ui/leafwiki-ui/src/features/users/{TOTPSetupDialog,TOTPDisableDialog}.tsx.
export default class TotpDialog {
  constructor(private page: Page) {}

  private async openAvatarMenu() {
    const avatar = this.page.getByTestId('user-toolbar-avatar');
    await avatar.waitFor({ state: 'visible' });
    await avatar.click();
  }

  async openEnableDialog() {
    await this.openAvatarMenu();
    const enableItem = this.page.getByTestId('user-toolbar-totp-enable');
    await enableItem.waitFor({ state: 'visible' });
    await enableItem.click();
  }

  async openDisableDialog() {
    await this.openAvatarMenu();
    const disableItem = this.page.getByTestId('user-toolbar-totp-disable');
    await disableItem.waitFor({ state: 'visible' });
    await disableItem.click();
  }

  // Step 1 of setup: confirm current password, advancing to the QR/code step.
  async submitSetupPassword(password: string) {
    const input = this.page.getByTestId('totp-setup-password');
    await input.waitFor({ state: 'visible' });
    await input.fill(password);
    await this.page.getByTestId('totp-setup-dialog-button-confirm').click();
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
    await this.page.getByTestId('totp-setup-dialog-button-confirm').click();
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

  // Final step of setup: dismiss the recovery-codes step, closing the dialog.
  async finishSetup() {
    await this.page.getByTestId('totp-setup-dialog-button-confirm').click();
  }

  // Disable dialog: password + a TOTP or recovery code in one step.
  async submitDisable(password: string, code: string) {
    await this.page.getByTestId('totp-disable-password').fill(password);
    await this.page.getByTestId('totp-disable-code').fill(code);
    await this.page.getByTestId('totp-disable-dialog-button-confirm').click();
  }
}
