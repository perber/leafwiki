import path from 'path';
import test, { expect } from '@playwright/test';
import EditPage from '../pages/EditPage';
import ImporterPage from '../pages/ImporterPage';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';
const importZipPath = path.resolve(__dirname, '../../internal/importer/fixtures/fixture-1.zip');
const importZipFileName = 'fixture-1.zip';
const importMetadataZipPath = path.resolve(
  __dirname,
  '../../internal/importer/fixtures/import-metadata.zip',
);
const importMetadataZipFileName = 'import-metadata.zip';

async function openFrontmatterPanel(page: import('@playwright/test').Page) {
  const trigger = page.locator('.page-frontmatter-panel__trigger');
  await trigger.waitFor({ state: 'visible' });
  await trigger.click();
  await page.locator('[data-testid="page-frontmatter-tag-input"]').waitFor({ state: 'visible' });
}

test.describe('Importer', () => {
  test.beforeEach(async ({ page }) => {
    const loginPage = new LoginPage(page);
    await loginPage.goto();
    await loginPage.login(user, password);

    const viewPage = new ViewPage(page);
    await viewPage.expectUserLoggedIn();
  });

  test.afterEach(async ({ page }) => {
    const viewPage = new ViewPage(page);
    await viewPage.logout();
  });

  test('create-and-clear-import-plan-from-zip', async ({ page }) => {
    const importerPage = new ImporterPage(page);
    await importerPage.goto();
    await importerPage.clearImportPlanIfPresent();

    await importerPage.uploadZip(importZipPath, importZipFileName);
    await importerPage.createImportPlan();

    await importerPage.expectPlanStatus('Planned');
    await importerPage.expectPlanItemCount(3);
    await importerPage.expectPlanContainsSourcePath('home.md');
    await importerPage.expectPlanContainsSourcePath('features/index.md');
    await importerPage.expectPlanContainsSourcePath('features/mermaind.md');

    await importerPage.clearImportPlan();
  });

  test('can-start-a-new-import-after-successful-import', async ({ page }) => {
    const importerPage = new ImporterPage(page);
    await importerPage.goto();
    await importerPage.clearImportPlanIfPresent();

    await importerPage.uploadZip(importZipPath, importZipFileName);
    await importerPage.createImportPlan();
    await importerPage.executeImportPlan();

    await importerPage.startNewImport();
  });

  test('close-and-clear-removes-completed-import-state', async ({ page }) => {
    const importerPage = new ImporterPage(page);
    await importerPage.goto();
    await importerPage.clearImportPlanIfPresent();

    await importerPage.uploadZip(importZipPath, importZipFileName);
    await importerPage.createImportPlan();
    await importerPage.executeImportPlan();

    await importerPage.closeAndClear();
    await importerPage.goto();
    await importerPage.expectNoStoredPlan();
  });

  test('import-shows-tags-and-properties-in-viewer-and-editor', async ({ page }) => {
    const importerPage = new ImporterPage(page);
    await importerPage.goto();
    await importerPage.clearImportPlanIfPresent();

    await importerPage.uploadZip(importMetadataZipPath, importMetadataZipFileName);
    await importerPage.createImportPlan();
    await importerPage.executeImportPlan();

    const viewPage = new ViewPage(page);
    await viewPage.goto('/home');

    await expect(
      page.locator('.page-metadata__tag-chip').filter({ hasText: 'imported-e2e-tag' }),
    ).toBeVisible();
    await expect(
      page.locator('.page-metadata__tag-chip').filter({ hasText: 'docs-import' }),
    ).toBeVisible();

    const propsToggle = page.locator('.page-metadata__props-toggle');
    await propsToggle.waitFor({ state: 'visible' });
    await propsToggle.click();

    await expect(
      page.locator('.page-metadata__prop-key').filter({ hasText: 'status' }),
    ).toBeVisible();
    await expect(
      page.locator('.page-metadata__prop-value').filter({ hasText: 'published' }),
    ).toBeVisible();
    await expect(
      page.locator('.page-metadata__prop-key').filter({ hasText: 'owner' }),
    ).toBeVisible();
    await expect(
      page.locator('.page-metadata__prop-value').filter({ hasText: 'importer-e2e' }),
    ).toBeVisible();

    await viewPage.clickEditPageButton();
    await openFrontmatterPanel(page);

    await expect(
      page.locator('.page-frontmatter-panel__chip').filter({ hasText: 'imported-e2e-tag' }),
    ).toBeVisible();
    await expect(
      page.locator('.page-frontmatter-panel__chip').filter({ hasText: 'docs-import' }),
    ).toBeVisible();

    const keyInputs = page.locator('[data-testid^="page-frontmatter-field-key-"]');
    const valueInputs = page.locator('[data-testid^="page-frontmatter-field-value-"]');
    await expect(keyInputs).toHaveCount(2);

    const keys = await keyInputs.evaluateAll((els: HTMLInputElement[]) => els.map((e) => e.value));
    const values = await valueInputs.evaluateAll((els: HTMLInputElement[]) =>
      els.map((e) => e.value),
    );

    expect(keys).toContain('status');
    expect(keys).toContain('owner');
    expect(values).toContain('published');
    expect(values).toContain('importer-e2e');

    const editPage = new EditPage(page);
    await editPage.closeEditor();
  });
});
