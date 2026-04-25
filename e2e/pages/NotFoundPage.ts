import { Page } from '@playwright/test';
import { expect } from '@playwright/test';
import { toAppPath } from './appPath';

export default class NotFoundPage {
  constructor(private page: Page) {}

  async goto(pagePath: string = '/') {
    await this.page.goto(toAppPath(pagePath));
  }

  async isNotFoundPage() {
    // h1 selector text "Page Not Found"
    const notFoundHeader = this.page.locator('h1');
    const text = await notFoundHeader.innerText();
    return text.includes('Page Not Found');
  }

  async getCreatePageButton() {
    return this.page.getByTestId('page404-create-page-button');
  }

  async clickCreatePageButton() {
    const createPageButton = await this.getCreatePageButton();
    await createPageButton.click();
  }

  async expectCreatePageButtonVisible() {
    await expect(this.getCreatePageButton()).toBeVisible();
  }

  async expectCreatePageButtonHidden() {
    await expect(this.getCreatePageButton()).toHaveCount(0);
  }
}
