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
const importedMetadataPagePath = '/imported-metadata-page';

async function expectViewerPropertyRow(
  page: import('@playwright/test').Page,
  key: string,
  value: string,
) {
  const row = page
    .locator('.page-metadata__prop-row')
    .filter({
      has: page.locator('.page-metadata__prop-key', { hasText: key }),
    })
    .filter({
      has: page.locator('.page-metadata__prop-value', { hasText: value }),
    });
  await expect(row).toBeVisible();
}

async function getEditorProperties(page: import('@playwright/test').Page) {
  const rows = page.locator('.page-frontmatter-panel__field-row');
  const count = await rows.count();
  const properties: Record<string, string> = {};

  for (let index = 0; index < count; index += 1) {
    const row = rows.nth(index);
    const key = await row.locator('[data-testid^="page-frontmatter-field-key-"]').inputValue();
    const value = await row.locator('[data-testid^="page-frontmatter-field-value-"]').inputValue();
    properties[key] = value;
  }

  return properties;
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
    await viewPage.goto(importedMetadataPagePath);

    await expect(
      page.locator('.page-metadata__tag-chip').filter({ hasText: 'imported-e2e-tag' }),
    ).toBeVisible();
    await expect(
      page.locator('.page-metadata__tag-chip').filter({ hasText: 'docs-import' }),
    ).toBeVisible();

    const propsToggle = page.locator('.page-metadata__props-toggle');
    await propsToggle.waitFor({ state: 'visible' });
    await propsToggle.click();

    await expectViewerPropertyRow(page, 'status', 'published');
    await expectViewerPropertyRow(page, 'owner', 'importer-e2e');

    await viewPage.clickEditPageButton();
    const editPage = new EditPage(page);
    await editPage.openFrontmatterPanel();

    await expect(
      page.locator('.page-frontmatter-panel__chip').filter({ hasText: 'imported-e2e-tag' }),
    ).toBeVisible();
    await expect(
      page.locator('.page-frontmatter-panel__chip').filter({ hasText: 'docs-import' }),
    ).toBeVisible();

    const keyInputs = page.locator('[data-testid^="page-frontmatter-field-key-"]');
    await expect(keyInputs).toHaveCount(2);

    const properties = await getEditorProperties(page);
    expect(properties).toEqual({
      owner: 'importer-e2e',
      status: 'published',
    });

    await editPage.closeEditor();
  });
});
