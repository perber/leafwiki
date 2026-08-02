import { Page } from '@playwright/test';
import { toAppPath } from './appPath';

export default class UserManagementPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto(toAppPath('/users'));
    await this.page.locator('table.settings__table').waitFor({ state: 'visible' });
  }

  // Role is left at the form's default (Editor) — irrelevant to what this
  // page object's callers verify (whether a user survives a live restore).
  async createUser(username: string, email: string, password: string) {
    await this.page.getByRole('button', { name: 'New User' }).click();

    const dialog = this.page.getByRole('dialog');
    await dialog.locator('input[name="username"]').fill(username);
    await dialog.locator('input[name="email"]').fill(email);
    await dialog.locator('input[name="new-password"]').fill(password);

    await dialog.locator('button[data-testid="user-form-dialog-button-confirm"]').click();
    await dialog.waitFor({ state: 'detached' });
  }

  userRow(username: string) {
    return this.page.locator('tr.settings__table-row', { hasText: username });
  }

  async expectUserPresent(username: string) {
    await this.userRow(username).waitFor({ state: 'visible' });
  }

  async expectUserAbsent(username: string) {
    await this.userRow(username).waitFor({ state: 'detached' });
  }
}
