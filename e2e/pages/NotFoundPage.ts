import { Page } from '@playwright/test';
import { expect } from '@playwright/test';
import { toAppPath } from './appPath';

export default class NotFoundPage {
  constructor(private page: Page) {}

  async goto(pagePath: string = '/') {
    await this.page.goto(toAppPath(pagePath));
  }

  async isNotFoundPage() {
    return this.page.getByTestId('page404').isVisible();
  }

  async expectVisible() {
    await expect(this.page.getByTestId('page404')).toBeVisible();
  }

  getCreatePageButton() {
    return this.page.getByTestId('page404-create-page-button');
  }

  async clickCreatePageButton() {
    const createPageButton = this.getCreatePageButton();
    await createPageButton.click();
  }

  async expectCreatePageButtonVisible() {
    await expect(this.getCreatePageButton()).toBeVisible();
  }

  async expectCreatePageButtonHidden() {
    await expect(this.getCreatePageButton()).toHaveCount(0);
  }
}
