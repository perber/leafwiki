import { Page } from '@playwright/test';

export default class CreatePageByPathDialog {
  constructor(private page: Page) {}

  async clickCreate() {
    const createButton = this.page.locator(
      'button[data-testid="create-page-by-path-dialog-button-confirm"]',
    );
    await createButton.waitFor({ state: 'visible' });
    await createButton.click();
    await createButton.waitFor({ state: 'detached' });
  }
}
