import { Page } from '@playwright/test';

export default class CreatePageByPathDialog {
  constructor(private page: Page) {}

  async clickCreate() {
    const createButton = this.page.locator(
      'button[data-testid="create-page-by-path-dialog-save-button"]',
    );
    await createButton.click();
    await this.page.waitForTimeout(600); // wait for creation to complete
  }
}
