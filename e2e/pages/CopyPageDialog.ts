import { expect, Page } from '@playwright/test';

export default class CopyPageDialog {
  constructor(private page: Page) {}

  async getTitleInput() {
    return this.page.locator('input[data-testid="copy-page-dialog-title-input"]');
  }

  async getSlugInput() {
    return this.page.locator('input[data-testid="copy-page-dialog-slug-input"]');
  }

  async getCreateButton() {
    return this.page.locator('button[data-testid="copy-page-dialog-button-confirm"]');
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

  async cancel() {
    const cancelButton = this.page.locator('button[data-testid="copy-page-dialog-button-cancel"]');
    await cancelButton.waitFor({ state: 'visible' });
    await cancelButton.click();
    await cancelButton.waitFor({ state: 'detached' });
  }
}
