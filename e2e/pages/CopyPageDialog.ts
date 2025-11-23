import { Page } from '@playwright/test';

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

    // Wait max 5 seconds for the slug to be auto-generated
    for (let i = 0; i < 50; i++) {
      const slugValue = await slugInput.inputValue();
      if (slugValue === expectedSlug) {
        return;
      }
      await this.page.waitForTimeout(100);
    }
    throw new Error(
      `Expected slug to be "${expectedSlug}", but got "${await slugInput.inputValue()}"`,
    );
  }

  async submitWithoutRedirect() {
    const createButton = await this.getCreateButton();
    await createButton.click();
    // Wait a 600 ms to ensure the dialog has processed the creation
    await this.page.waitForTimeout(600);
  }
}
