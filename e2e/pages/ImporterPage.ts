import { expect, Page } from '@playwright/test';
import { toAppPath } from './appPath';

export default class ImporterPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto(toAppPath('/settings/importer'));
    await expect(
      this.page.getByRole('heading', { name: 'Import', exact: true, level: 1 }),
    ).toBeVisible();
  }

  async uploadZip(filePath: string, fileName: string) {
    await this.page.locator('input[type="file"][accept=".zip"]').setInputFiles(filePath);
    await expect(this.page.getByText(fileName)).toBeVisible();
  }

  async createImportPlan() {
    await this.page.getByRole('button', { name: 'Import from Zip' }).click();
    await expect(this.page.getByText('Import plan created successfully')).toBeVisible();
    await expect(this.page.getByRole('heading', { name: 'Import Plan' })).toBeVisible();
  }

  async expectPlanStatus(status: 'Planned' | 'Running' | 'Completed' | 'Canceled' | 'Failed') {
    await expect(this.page.locator('.importer__status-title')).toHaveText(status);
  }

  async expectPlanItemCount(count: number) {
    await expect(this.page.getByRole('heading', { name: `Planned Items (${count})` })).toBeVisible();
  }

  async expectPlanContainsSourcePath(sourcePath: string) {
    await expect(this.page.getByRole('cell', { name: sourcePath })).toBeVisible();
  }

  async clearImportPlan() {
    await this.page.getByRole('button', { name: 'Clear Import Plan' }).click();
    await expect(this.page.getByText('Import plan cleared')).toBeVisible();
    await expect(this.page.getByRole('heading', { name: 'Import Plan' })).toHaveCount(0);
  }

  async clearImportPlanIfPresent() {
    const clearButton = this.page.getByRole('button', { name: 'Clear Import Plan' });
    if (await clearButton.count()) {
      await clearButton.click();
      await expect(this.page.getByText('Import plan cleared')).toBeVisible();
      await expect(this.page.getByRole('heading', { name: 'Import Plan' })).toHaveCount(0);
    }
  }
}
