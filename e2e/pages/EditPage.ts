import { Page } from '@playwright/test';

export default class EditPage {
  constructor(private page: Page) {}

  async writeContent(content: string) {
    // Code mirror editor
    const editor = this.page.locator('.cm-editor');
    await editor.click();
    await this.page.keyboard.type(content);
  }

  async savePage() {
    const saveButton = this.page.locator('button[data-testid="save-page-button"]');
    await saveButton.click();
    await this.page.waitForTimeout(500); // wait for save to complete
  }

  async closeEditor() {
    const closeButton = this.page.locator('button[data-testid="close-editor-button"]');
    await closeButton.click();
  }

  async openAssetManager() {
    const assetManagerButton = this.page.locator('button[data-testid="open-asset-manager-button"]');
    await assetManagerButton.click();
  }

  async uploadAsset(filePath: string) {
    const dropzone = this.page.locator('div[data-testid="asset-upload-dropzone"]');

    const [fileChooser] = await Promise.all([
      this.page.waitForEvent('filechooser'),
      dropzone.click(), // trigger the picker
    ]);

    await fileChooser.setFiles(filePath);
    // wait until the asset appears in the list
    await this.page
      .locator('li[data-testid="asset-item"]')
      .first()
      .waitFor({ state: 'visible', timeout: 5000 });
  }

  async listAmountOfAssets(): Promise<number> {
    const assets = this.page.locator('li[data-testid="asset-item"]');
    return assets.count();
  }

  async insertFirstAssetIntoPage() {
    const firstAsset = this.page.locator('li[data-testid="asset-item"]').first();
    await firstAsset.dblclick();
  }
}
