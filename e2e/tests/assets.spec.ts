import path from 'path';
import test from '@playwright/test';
import AddPageDialog from '../pages/AddPageDialog';
import EditPage from '../pages/EditPage';
import LoginPage from '../pages/LoginPage';
import TreeView from '../pages/TreeView';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';
const uploadAssetPath = path.resolve(__dirname, '../assets/upload-test.png');

test.describe('Asset Uploads', () => {
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

  test('upload-asset-and-render-it-on-page', async ({ page }) => {
    const title = `Asset Page ${Date.now()}`;

    const treeView = new TreeView(page);
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(title);
    await addPageDialog.submitWithoutRedirect();
    await treeView.clickPageByTitle(title);

    const viewPage = new ViewPage(page);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.openAssetManager();
    await editPage.uploadAsset(uploadAssetPath);
    test.expect(await editPage.listAmountOfAssets()).toBe(1);

    await editPage.insertFirstAssetIntoPage();
    await editPage.savePage();
    await editPage.closeEditor();

    test.expect(await viewPage.amountOfImages()).toBe(1);
  });
});
