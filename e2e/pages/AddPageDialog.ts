import { expect, Page } from '@playwright/test';

export default class LoginPage {
  constructor(private page: Page) {}

  async getTitleInput() {
    return this.page.locator('input[data-testid="add-page-title-input"]');
  }

  async getSlugInput() {
    return this.page.locator('input[data-testid="add-page-slug-input"]');
  }

  async getCreateButton() {
    return this.page.locator('button[data-testid="add-page-dialog-button-no-redirect"]');
  }

  async fillTitle(title: string) {
    const titleInput = await this.getTitleInput();
    const slugInput = await this.getSlugInput();

    await titleInput.fill(title);

    const expectedSlug = title
      .toLowerCase()
      .replace(/\s+/g, '-')
      .replace(/[^\w-]/g, '');

    await expect
      .poll(async () => slugInput.inputValue(), {
        message: `Expected slug to be "${expectedSlug}"`,
      })
      .toBe(expectedSlug);
  }

  async submitWithoutRedirect() {
    const createButton = await this.getCreateButton();
    await createButton.waitFor({ state: 'visible' });
    await createButton.click();
    await createButton.waitFor({ state: 'detached' });
  }
}
