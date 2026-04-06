import { Page } from '@playwright/test';

export default class EditPage {
  constructor(private page: Page) {}

  async writeContent(content: string) {
    const editor = this.page.locator('.cm-editor');
    await editor.click();
    await this.page.keyboard.type(content);
  }

  async savePage() {
    const saveButton = this.page.locator('button[data-testid="save-page-button"]');
    await saveButton.click();
    await this.page.waitForTimeout(500);
  }

  async closeEditor() {
    const closeButton = this.page.locator('button[data-testid="close-editor-button"]');
    await closeButton.click();
  }

  async openAssetManager() {
    const assetManagerButton = this.page.locator('button[data-testid="open-asset-manager-button"]');
    await assetManagerButton.click();
  }

  async openMetadataDialog() {
    const metadataButton = this.page.locator('button[data-testid="edit-page-metadata-button"]');
    await metadataButton.click();
  }

  async uploadAsset(filePath: string) {
    const dropzone = this.page.locator('div[data-testid="asset-upload-dropzone"]');

    const [fileChooser] = await Promise.all([
      this.page.waitForEvent('filechooser'),
      dropzone.click(),
    ]);

    await fileChooser.setFiles(filePath);
    await this.page.locator('li[data-testid="asset-item"]').first().waitFor({ state: 'visible' });
  }

  async listAmountOfAssets(): Promise<number> {
    const assets = this.page.locator('li[data-testid="asset-item"]');
    return assets.count();
  }

  async insertFirstAssetIntoPage() {
    const firstAsset = this.page.locator('li[data-testid="asset-item"]').first();
    await firstAsset.dblclick();
  }

  async insertAssetAsPlayer(filename: string) {
    const asset = this.page
      .locator('li[data-testid="asset-item"]')
      .filter({ hasText: filename });
    await asset.locator('[data-testid="asset-insert-player-button"]').click();
  }

  async insertAssetAsLink(filename: string) {
    const asset = this.page
      .locator('li[data-testid="asset-item"]')
      .filter({ hasText: filename });
    await asset.locator('[data-testid="asset-insert-link-button"]').click();
  }
}
