import { expect, Page } from '@playwright/test';

export default class EditPageMetadataDialog {
  constructor(private page: Page) {}

  async titleInput() {
    return this.page.locator('input[data-testid="edit-page-metadata-dialog-title-input"]');
  }

  async slugInput() {
    return this.page.locator('input[data-testid="edit-page-metadata-dialog-slug-input"]');
  }

  async pathDisplay() {
    return this.page.locator('[data-testid="edit-page-metadata-dialog-path-display"]');
  }

  async fillTitle(title: string) {
    const input = await this.titleInput();
    await input.fill(title);
  }

  async fillSlug(slug: string) {
    const input = await this.slugInput();
    await input.fill(slug);
  }

  async expectPath(path: string) {
    await expect(await this.pathDisplay()).toHaveText(`Path: ${path}`);
  }

  async expectSlug(slug: string) {
    await expect(await this.slugInput()).toHaveValue(slug);
  }

  async submit() {
    const button = this.page.locator(
      'button[data-testid="edit-page-metadata-dialog-button-confirm"]',
    );
    await button.click();
  }
}
