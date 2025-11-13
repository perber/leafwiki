import { Page } from '@playwright/test';

export default class AssetManager {
    constructor(private page: Page) { }
    async getUploadInput() {
        return this.page.locator('input[data-testid="asset-upload-input"]');
    }
    
    async uploadFile(filePath: string) {
        const uploadInput = await this.getUploadInput();
        await uploadInput.setInputFiles(filePath);
        // Wait for upload to complete
        await this.page.waitForSelector('div[data-testid="asset-upload-complete"]', { state: 'visible' });
    }
}
