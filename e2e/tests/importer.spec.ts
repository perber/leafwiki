import path from 'path';
import test from '@playwright/test';
import ImporterPage from '../pages/ImporterPage';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';
const importZipPath = path.resolve(__dirname, '../../internal/importer/fixtures/fixture-1.zip');
const importZipFileName = 'fixture-1.zip';

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
});
