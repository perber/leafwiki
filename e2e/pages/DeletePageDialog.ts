import { expect, Page } from '@playwright/test';

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

  /**
   * Clicks the confirm button and waits for the DELETE API call to complete,
   * without waiting for the dialog to close. Use this when the delete is
   * expected to fail (e.g. non-recursive delete of a page with children).
   */
  async tryConfirmDeletion() {
    const deleteButton = this.page.locator(
      'button[data-testid="delete-page-dialog-button-confirm"]',
    );
    await deleteButton.waitFor({ state: 'visible' });
    await Promise.all([
      this.page.waitForResponse(
        (r) => r.url().includes('/api/pages/') && r.request().method() === 'DELETE',
      ),
      deleteButton.click(),
    ]);
  }

  async expectBacklinksWarningVisible() {
    await expect(this.page.getByTestId('delete-page-dialog-backlinks-warning')).toBeVisible();
  }

  async expectNoBacklinksVisible() {
    await expect(this.page.getByTestId('delete-page-dialog-no-backlinks')).toBeVisible();
  }

  async expectBacklinkTitle(title: string) {
    await expect(this.page.getByTestId('delete-page-dialog-backlinks-list')).toContainText(title);
  }
}
