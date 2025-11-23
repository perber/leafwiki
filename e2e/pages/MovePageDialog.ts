import { Page } from '@playwright/test';

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
}
