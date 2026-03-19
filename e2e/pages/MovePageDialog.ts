import { expect, Page } from '@playwright/test';

export default class MovePageDialog {
  constructor(private page: Page) {}

  async getParentSelection() {
    return this.page.locator('button[role="combobox"]');
  }

  async selectNewParentAsTopLevel() {
    const parentSelection = await this.getParentSelection();
    await parentSelection.click();
    // find by text contains "Top Level" should be regex because at the beginning there is an emoji
    const option = this.page
      .locator(`div[role="option"]`)
      .filter({ hasText: new RegExp('Top Level') })
      .first();
    await option.click();
  }

  async clickMoveButton() {
    const moveButton = this.page.locator('button[data-testid="move-page-dialog-button-confirm"]');
    await moveButton.click();
  }

  async expectRefactorDialogVisible() {
    await expect(
      this.page.locator('button[data-testid="page-refactor-dialog-button-confirm"]'),
    ).toBeVisible();
  }

  async expectRefactorDialogHidden() {
    await expect(
      this.page.locator('button[data-testid="page-refactor-dialog-button-confirm"]'),
    ).toHaveCount(0);
  }

  async confirmRefactorDialog() {
    await this.page.locator('button[data-testid="page-refactor-dialog-button-confirm"]').click();
  }

  async expectAffectedPagesCount(count: number) {
    await expect(
      this.page.locator('[data-testid="page-refactor-dialog-affected-page"]'),
    ).toHaveCount(count);
  }

  async expectAffectedPageTitle(title: string) {
    await expect(
      this.page
        .locator('[data-testid="page-refactor-dialog-affected-page"]')
        .filter({ hasText: title }),
    ).toHaveCount(1);
  }
}
