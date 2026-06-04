import { expect, Page } from '@playwright/test';

export default class EditPage {
  constructor(private page: Page) {}

  async writeContent(content: string) {
    const editor = this.page.locator('.cm-editor');
    await editor.click();
    await this.page.keyboard.type(content);
  }

  async openReplacePanel() {
    const editor = this.page.locator('.cm-editor');
    await editor.click();
    await this.page.keyboard.press('Control+h');
    await this.page.locator('.cm-search input[main-field="true"]').waitFor({ state: 'visible' });
  }

  async replaceAll(search: string, replace: string) {
    const searchInput = this.page.locator('.cm-search input[main-field="true"]');
    const replaceInput = this.page.locator('.cm-search input[name="replace"]');

    await searchInput.fill(search);
    await replaceInput.fill(replace);
    await this.page.locator('.cm-search button[name="replaceAll"]').click();
  }

  async closeSearchPanelWithEscape() {
    await this.page.keyboard.press('Escape');
    await this.page.locator('.cm-search').waitFor({ state: 'hidden' });
  }

  async expectEditorStillOpen() {
    await this.page.locator('.cm-editor').waitFor({ state: 'visible' });
  }

  async savePage() {
    const saveButton = this.page.locator('button[data-testid="save-page-button"]');
    await saveButton.waitFor({ state: 'visible' });
    await saveButton.click();
    await this.page.getByText('Page saved successfully').last().waitFor({ state: 'visible' });
  }

  async closeEditor() {
    const closeButton = this.page.locator('button[data-testid="close-editor-button"]');
    await closeButton.click();
    await this.page.waitForLoadState('networkidle');
  }

  async clickLeaveAnyway() {
    const leaveButton = this.page.locator(
      'button[data-testid="unsaved-changes-dialog-button-confirm"]',
    );
    await leaveButton.waitFor({ state: 'visible' });
    await leaveButton.click();
    await this.page.waitForLoadState('networkidle');
  }

  async clickUnsavedChangesCancel() {
    const cancelButton = this.page.locator(
      'button[data-testid="unsaved-changes-dialog-button-cancel"]',
    );
    await cancelButton.waitFor({ state: 'visible' });
    await cancelButton.click();
  }

  async openAssetManager() {
    const assetManagerButton = this.page.locator('button[data-testid="open-asset-manager-button"]');
    await assetManagerButton.click();
    await this.page
      .locator('div[data-testid="asset-upload-dropzone"]')
      .waitFor({ state: 'visible' });
    await this.page.locator('.asset-manager__loading').waitFor({ state: 'hidden' });
  }

  async openMetadataDialog() {
    const metadataButton = this.page.locator('button[data-testid="edit-page-metadata-button"]');
    await metadataButton.click();
  }

  async openFrontmatterPanel() {
    const trigger = this.page.locator('.page-frontmatter-panel__trigger');
    await trigger.waitFor({ state: 'visible' });
    await trigger.click();
    await this.page.locator('[data-testid="page-frontmatter-tag-input"]').waitFor({
      state: 'visible',
    });
  }

  async uploadAsset(filePath: string) {
    const dropzone = this.page.locator('div[data-testid="asset-upload-dropzone"]');
    const assets = this.page.locator('li[data-testid="asset-item"]');
    const existingCount = await assets.count();

    const [fileChooser] = await Promise.all([
      this.page.waitForEvent('filechooser'),
      dropzone.click(),
    ]);

    await fileChooser.setFiles(filePath);
    await expect(assets).toHaveCount(existingCount + 1);
  }

  async listAmountOfAssets(): Promise<number> {
    await this.page.locator('.asset-manager__loading').waitFor({ state: 'hidden' });
    const assets = this.page.locator('li[data-testid="asset-item"]');
    return assets.count();
  }

  async insertFirstAssetIntoPage() {
    const firstAsset = this.page.locator('li[data-testid="asset-item"]').first();
    await firstAsset.dblclick();
  }

  async insertAssetAsPlayer(filename: string) {
    const asset = this.page.locator('li[data-testid="asset-item"]').filter({ hasText: filename });
    await asset.locator('[data-testid="asset-insert-player-button"]').click();
  }

  async insertAssetAsLink(filename: string) {
    const asset = this.page.locator('li[data-testid="asset-item"]').filter({ hasText: filename });
    await asset.locator('[data-testid="asset-insert-link-button"]').click();
  }

  async deleteFirstAsset() {
    const firstAsset = this.page.locator('li[data-testid="asset-item"]').first();
    const assets = this.page.locator('li[data-testid="asset-item"]');
    const existingCount = await assets.count();
    await firstAsset.locator('button[title="Delete"]').click();
    await expect(assets).toHaveCount(existingCount - 1);
  }

  async closeAssetManager() {
    await this.page.keyboard.press('Escape');
    await this.page.locator('div[role="dialog"]').waitFor({ state: 'hidden' });
  }

  async waitForAutocompleteDropdown() {
    await this.page.locator('.cm-tooltip-autocomplete').waitFor({ state: 'visible' });
  }

  async selectAutocompleteOption(label: string) {
    await this.page
      .locator('.cm-tooltip-autocomplete .cm-completionLabel', { hasText: label })
      .click();
  }

  async getEditorContent(): Promise<string> {
    return this.page.locator('.cm-content').innerText();
  }
}
