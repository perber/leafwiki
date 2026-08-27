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

  pageTitle() {
    return this.page.locator('h1.settings__title');
  }

  languageSelect() {
    return this.page.locator('button[data-testid="preferences-language-select"]');
  }

  private languageOptions() {
    return this.page.locator('[role="option"]');
  }

  async selectLanguage(name: string) {
    await this.languageSelect().click();
    await this.languageOptions().filter({ hasText: name }).first().click();
  }

  dateFormatSelect() {
    return this.page.locator('button[data-testid="preferences-dateformat-select"]');
  }

  timeFormatSelect() {
    return this.page.locator('button[data-testid="preferences-timeformat-select"]');
  }

  async selectDateFormat(label: string) {
    await this.dateFormatSelect().click();
    await this.page.locator('[role="option"]').filter({ hasText: label }).first().click();
  }

  async selectTimeFormat(label: string) {
    await this.timeFormatSelect().click();
    await this.page.locator('[role="option"]').filter({ hasText: label }).first().click();
  }
}
