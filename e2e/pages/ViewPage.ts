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
}
