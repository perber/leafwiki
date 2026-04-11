import { Page } from '@playwright/test';

export default class DeletePageDialog {
  constructor(private page: Page) {}

  async dialogTextVisible() {
    return this.page
      .getByText(
        /Are you sure you want to delete this (page|section)\? This action cannot be undone\./,
      )
      .isVisible();
  }

  async checkboxRecursiveDelete() {
    return this.page.locator('button[data-testid="delete-page-dialog-recursive-delete-checkbox"]');
  }

  async confirmNestedDeletion() {
    const checkbox = await this.checkboxRecursiveDelete();
    await checkbox.click();
    await this.confirmDeletion();
  }

  async abortDeletion() {
    const cancelButton = this.page.locator(
      'button[data-testid="delete-page-dialog-button-cancel"]',
    );
    await cancelButton.waitFor({ state: 'visible' });
    await cancelButton.click();
    await cancelButton.waitFor({ state: 'detached' });
  }

  async confirmDeletion() {
    const deleteButton = this.page.locator(
      'button[data-testid="delete-page-dialog-button-confirm"]',
    );
    await deleteButton.waitFor({ state: 'visible' });
    await deleteButton.click();
    await deleteButton.waitFor({ state: 'detached' });
  }
}
