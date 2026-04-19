import { Page } from '@playwright/test';
import { toAppPath } from './appPath';
import { expect } from '@playwright/test';

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
    const historyButton = this.page.getByTestId('page-history-button');
    const overflowButton = this.page.getByTestId('toolbar-overflow-button');

    await expect
      .poll(async () => {
        const historyVisible = await historyButton.isVisible().catch(() => false);
        const overflowVisible = await overflowButton.isVisible().catch(() => false);
        return historyVisible || overflowVisible;
      })
      .toBe(true);

    if (await historyButton.isVisible().catch(() => false)) {
      await historyButton.click();
      return;
    }

    await this.openToolbarOverflow();
    const historyMenuItem = this.page.getByTestId('page-history-menu-item');
    await historyMenuItem.waitFor({ state: 'visible' });
    await historyMenuItem.click();
  }

  async openCurrentPageHistory() {
    await this.clickPageHistoryButton();
    await this.page.locator('[data-testid="page-history-page-content"]').waitFor({
      state: 'visible',
    });
  }

  // The revision list is now an inline left panel on the history page — there
  // is no separate sidebar tab to click. This method waits for the list panel
  // to be ready, which it always is once the history page is open.
  async switchToRevisionsTab() {
    await this.expectRevisionListVisible();
  }

  async expectRevisionListVisible() {
    await this.page.locator('[data-testid="page-history-page-list"]').waitFor({ state: 'visible' });
  }

  // Kept for backward compatibility — alias of expectRevisionListVisible.
  async expectRevisionsSidebarOpen() {
    await this.expectRevisionListVisible();
  }

  async openFirstRevision() {
    const firstRevision = this.page
      .locator('button[data-testid^="history-sidebar-revision-"]')
      .first();
    await firstRevision.waitFor({ state: 'visible' });
    await firstRevision.click();
  }

  async openRevisionAt(index: number) {
    const revision = this.page
      .locator('button[data-testid^="history-sidebar-revision-"]')
      .nth(index);
    await revision.waitFor({ state: 'visible' });
    await revision.click();
  }

  async expectRevisionViewOpen() {
    await this.page
      .locator('[data-testid="page-history-page-content"]')
      .waitFor({ state: 'visible' });
    await this.expectRevisionsSidebarOpen();
    await this.page.locator('button[data-testid="back-to-page-button"]').waitFor({
      state: 'visible',
    });
  }

  async returnToPage() {
    const backButton = this.page.locator('button[data-testid="back-to-page-button"]');
    await backButton.click();
  }

  async switchToHistoryPreviewTab() {
    await this.page.locator('[data-testid="page-history-page-preview-tab"]').click();
    await this.page.locator('[data-testid="page-history-page-content"]').waitFor({
      state: 'visible',
    });
  }

  async expectHistoryPreviewImageLoaded() {
    const image = this.page.locator('[data-testid="page-history-page-content"] img').first();
    await image.waitFor({ state: 'visible' });
    await expect
      .poll(async () => {
        return image.evaluate((img) => img.complete && img.naturalWidth > 0);
      })
      .toBe(true);
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

  async switchToExplorerTab() {
    const explorerTabButton = this.page.locator('button[data-testid="sidebar-tree-tab-button"]');
    await explorerTabButton.click();
    await this.page.locator('a[data-testid^="tree-node-link-"]').first().waitFor({
      state: 'visible',
    });
  }
}
