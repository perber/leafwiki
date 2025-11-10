import { Page } from '@playwright/test';

export default class DeletePageDialog {
    constructor(private page: Page) { }

    async dialogTextVisible() {
        // Are you sure you want to delete this page? This action cannot be undone.
        return this.page.locator('text=Are you sure you want to delete this page? This action cannot be undone.').isVisible();
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
        const cancelButton = this.page.locator('button[data-testid="delete-page-dialog-cancel-button"]');
        await cancelButton.click();
        // Wait a 600 ms to ensure the dialog has processed the deletion
        await this.page.waitForTimeout(600);
    }

    async confirmDeletion() {
        const deleteButton = this.page.locator('button[data-testid="delete-page-dialog-save-button"]');
        await deleteButton.click();
        // We will be redirected to another page, so wait a bit
        await this.page.waitForTimeout(1000);
    }
}
