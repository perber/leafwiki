import { Page } from "@playwright/test";


export default class ViewPage {
  constructor(private page: Page) {}

  async goto(pagePath: string = '/') {
    await this.page.goto(`${pagePath}`);
  }

  async isUserLoggedIn(): Promise<boolean> {
    return this.page.getByTestId('user-toolbar-avatar').isVisible();
  }

  async expectUserLoggedIn() {
    await this.page.getByTestId('user-toolbar-avatar').waitFor({ state: 'visible' });
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

  async clickEditPageButton() {
    const editButton = this.page.locator('button[data-testid="edit-page-button"]');
    await editButton.click();
  }

  async getContent() {
    return this.page.locator('article').innerText();
  }

  async amountOfSVGElements(): Promise<number> {
    await this.page.waitForSelector('article svg', { state: 'visible' });
    return this.page.locator('article svg').count();
  }

  async switchToSearchTab() {
    const searchTabButton = this.page.locator('button[data-testid="sidebar-search-tab-button"]');
    await searchTabButton.click();
    // wait for search input to be visible
    await this.page.locator('input[data-testid="search-input"]').waitFor({ state: 'visible' });
  }
}
