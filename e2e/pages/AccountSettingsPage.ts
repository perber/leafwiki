import { Page } from '@playwright/test';
import { toAppPath } from './appPath';

export default class AccountSettingsPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto(toAppPath('/settings/account'));
    await this.autoSaveCheckbox().waitFor({ state: 'visible' });
  }

  autoSaveCheckbox() {
    return this.page.locator('button[data-testid="preferences-autosave-checkbox"]');
  }

  async toggleAutoSave() {
    await this.autoSaveCheckbox().click();
  }
}
