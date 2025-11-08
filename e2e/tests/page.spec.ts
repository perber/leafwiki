import test from '@playwright/test';
import AddPageDialog from '../pages/AddPageDialog';
import DeletePageDialog from '../pages/DeletePageDialog';
import LoginPage from '../pages/LoginPage';
import TreeView from '../pages/TreeView';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

test.describe('Authenticated', () => {
  test.beforeEach(async ({ page }) => {
    const loginPage = new LoginPage(page);
    await loginPage.goto();
    await loginPage.login(user, password);
    const viewPage = new ViewPage(page);
    await viewPage.expectUserLoggedIn();
  });

  test('create-page', async ({ page }) => {
    const title = `My New Page ${Date.now()}`;

    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(title);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);
  });

  test('view-page', async ({ page }) => {
    const title = `Page To View ${Date.now()}`;

    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(title);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);
    await treeView.clickPageByTitle(title);

    const viewPage = new ViewPage(page);
    const pageTitle = await viewPage.getTitle();
    test.expect(pageTitle).toBe(title);
  });

  test('delete-page', async ({ page }) => {
    const title = `Page To Delete ${Date.now()}`;

    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(title);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);
    await treeView.clickPageByTitle(title);

    const viewPage = new ViewPage(page);
    const pageTitle = await viewPage.getTitle();
    test.expect(pageTitle).toBe(title);

    await viewPage.clickDeletePageButton();

    const deletePageDialog = new DeletePageDialog(page);
    test.expect(await deletePageDialog.dialogTextVisible()).toBeTruthy();
    await deletePageDialog.abortDeletion();
    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);

    await viewPage.clickDeletePageButton();
    test.expect(await deletePageDialog.dialogTextVisible()).toBeTruthy();
    await deletePageDialog.confirmDeletion();
    await treeView.expectNumberOfTreeNodes(curNodeCount);
  });
});
