import { expect, Page } from '@playwright/test';

export default class MovePageDialog {
  constructor(private page: Page) {}

  private dialog() {
    return this.page.locator('[role="dialog"]').last();
  }

  async getParentSelection() {
    return this.dialog().locator('[role="combobox"]');
  }

  private getOptions() {
    return this.dialog().locator('[role="option"]');
  }

  async selectNewParentAsTopLevel() {
    const parentSelection = await this.getParentSelection();
    await parentSelection.click();
    // find by text contains "Top Level" should be regex because at the beginning there is an emoji
    const option = this.getOptions()
      .filter({ hasText: new RegExp('Top Level') })
      .first();
    await option.click();
  }

  async selectNewParent(title: string) {
    const parentSelection = await this.getParentSelection();
    await parentSelection.click();
    const option = this.getOptions().filter({ hasText: title }).first();
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
