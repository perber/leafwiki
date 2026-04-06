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
const uploadAudioPath = path.resolve(__dirname, '../assets/upload-test.mp3');
const uploadVideoPath = path.resolve(__dirname, '../assets/upload-test.mp4');

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

  test('upload-image-asset-and-insert-it-as-link', async ({ page }) => {
    const title = `Linked Image Page ${Date.now()}`;

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
    await editPage.insertAssetAsLink('upload-test.png');

    await editPage.savePage();
    await editPage.closeEditor();

    test.expect(await page.locator('article img').count()).toBe(0);
    await page.locator('article a[href*="upload-test.png"]').waitFor({ state: 'visible' });
    test.expect(
      await page.locator('article a[href*="upload-test.png"]').count(),
    ).toBe(1);
  });

  test('upload-audio-and-video-assets-and-insert-players', async ({ page }) => {
    const title = `Media Asset Page ${Date.now()}`;

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
    await editPage.uploadAsset(uploadAudioPath);
    await editPage.insertAssetAsPlayer('upload-test.mp3');

    await editPage.openAssetManager();
    await editPage.uploadAsset(uploadVideoPath);
    test.expect(await editPage.listAmountOfAssets()).toBe(2);
    await editPage.insertAssetAsPlayer('upload-test.mp4');

    await editPage.savePage();
    await editPage.closeEditor();

    await page.locator('article audio[controls]').waitFor({ state: 'visible' });
    await page.locator('article video[controls]').waitFor({ state: 'visible' });
    test.expect(await page.locator('article audio[controls]').count()).toBe(1);
    test.expect(await page.locator('article video[controls]').count()).toBe(1);
  });
});
