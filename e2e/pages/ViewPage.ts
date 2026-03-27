import { Page } from '@playwright/test';
import { toAppPath } from './appPath';

export default class ViewPage {
  constructor(private page: Page) {}

  async goto(pagePath: string = '/') {
    await this.page.goto(toAppPath(pagePath));
    await this.page.locator('article').waitFor({ state: 'visible' });
  }

  async isUserLoggedIn(): Promise<boolean> {
    const avatar = this.page.getByTestId('user-toolbar-avatar');
    try {
      return await avatar.isVisible({ timeout: 1000 });
    } catch {
      return false;
    }
  }

  async expectUserLoggedIn() {
    await this.page.getByTestId('user-toolbar-avatar').waitFor({ state: 'visible' });
  }

  async clickUserToolbarAvatar() {
    const avatar = this.page.getByTestId('user-toolbar-avatar');
    await avatar.waitFor({ state: 'visible' });
    await avatar.click();
  }

  async logout() {
    const loginField = this.page.locator('input[data-testid="login-identifier"]');

    // Already logged out?
    try {
      if (await loginField.isVisible({ timeout: 1000 })) {
        return;
      }
    } catch {
      // if the locator does not exist yet, ignore
    }

    const avatar = this.page.getByTestId('user-toolbar-avatar');

    // wait for avatar to be visible
    try {
      if (!(await avatar.isVisible({ timeout: 2000 }))) {
        // not logged in / wrong page / page already gone
        return;
      }
    } catch {
      // not logged in / wrong page / page already gone
      return;
    }

    // 3) Open dropdown
    await avatar.click();

    // 4) Click logout button
    const logoutButton = this.page.getByTestId('user-toolbar-logout');
    await logoutButton.waitFor({ state: 'visible', timeout: 2000 });
    await logoutButton.click();

    // 5) Wait for login field again
    await loginField.waitFor({ state: 'visible' });
  }

  async isLoggedOut() {
    const loginField = this.page.locator('input[data-testid="login-identifier"]');
    try {
      return await loginField.isVisible({ timeout: 1000 });
    } catch {
      return false;
    }
  }

  async getTitle() {
    return this.page.locator('article>h1').innerText();
  }

  async clickDeletePageButton() {
    const deleteButton = this.page.locator('button[data-testid="delete-page-button"]');
    await deleteButton.click();
  }

  async clickCopyPageButton() {
    const copyButton = this.page.locator('button[data-testid="copy-page-button"]');
    await copyButton.click();
  }

  async openToolbarOverflow() {
    const overflowButton = this.page.getByTestId('toolbar-overflow-button');
    await overflowButton.waitFor({ state: 'visible' });
    await overflowButton.click();
  }

  async clickDeletePageMenuItem() {
    await this.openToolbarOverflow();
    const deleteMenuItem = this.page.getByTestId('delete-page-menu-item');
    await deleteMenuItem.waitFor({ state: 'visible' });
    await deleteMenuItem.click();
  }

  async clickCopyPageMenuItem() {
    await this.openToolbarOverflow();
    const copyMenuItem = this.page.getByTestId('copy-page-menu-item');
    await copyMenuItem.waitFor({ state: 'visible' });
    await copyMenuItem.click();
  }

  async clickPageHistoryButton() {
    const historyButton = this.page.locator('button[data-testid="page-history-button"]');
    await historyButton.click();
  }

  async clickEditPageButton() {
    const editButton = this.page.locator('button[data-testid="edit-page-button"]');
    await editButton.click();
    // wait for editor to load
    await this.page.locator('.cm-editor').waitFor({ state: 'visible' });
  }

  async getContent() {
    await this.page.locator('article').waitFor({ state: 'visible' });
    return this.page.locator('article').innerText();
  }

  async amountOfSVGElements(): Promise<number> {
    await this.page.locator('article .my-4 svg').waitFor({ state: 'visible' });
    return this.page.locator('article .my-4 svg').count();
  }

  async amountOfImages(): Promise<number> {
    await this.page.locator('article img').waitFor({ state: 'visible' });
    return this.page.locator('article img').count();
  }

  async switchToSearchTab() {
    const searchTabButton = this.page.locator('button[data-testid="sidebar-search-tab-button"]');
    await searchTabButton.click();
    // wait for search input to be visible
    await this.page.locator('input[data-testid="search-input"]').waitFor({ state: 'visible' });
  }
}
