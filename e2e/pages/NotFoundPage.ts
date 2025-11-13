import { Page } from '@playwright/test';

export default class NotFoundPage {
  constructor(private page: Page) {}

  async goto(pagePath: string = '/') {
    await this.page.goto(`${pagePath}`);
  }

  async isNotFoundPage() {
    // h1 selector text "Page Not Found"
    const notFoundHeader = this.page.locator('h1');
    const text = await notFoundHeader.innerText();
    return text.includes('Page Not Found');
  }

  async getCreatePageButton() {
    // main button selector
    return this.page.locator('main button').first();
  }

  async clickCreatePageButton() {
    const createPageButton = await this.getCreatePageButton();
    await createPageButton.click();
  }
}
