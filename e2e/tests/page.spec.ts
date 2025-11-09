import test from '@playwright/test';
import AddPageDialog from '../pages/AddPageDialog';
import CopyPageDialog from '../pages/CopyPageDialog';
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

  test('create-subpage', async ({ page }) => {
    const parentTitle = `Parent Page ${Date.now()}`;

    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(parentTitle);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);
    await treeView.createSubPageOfParent(parentTitle, `Child Page of ${parentTitle}`);
    await treeView.expectNumberOfTreeNodes(curNodeCount + 2);
  });

  test('sort-pages', async ({ page }) => {
    const parentTitle = `Sort Parent Page ${Date.now()}`;
    const childPages = ['Banana', 'Apple', 'Cherry', 'Date'];
    const desiredOrder = ['Apple', 'Banana', 'Cherry', 'Date'];

    // Create parent page
    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(parentTitle);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);

    // Create child pages
    await treeView.createMultipleSubPagesOfParent(parentTitle, childPages);
    await treeView.expectNumberOfTreeNodes(curNodeCount + childPages.length + 1);

    // Sort child pages
    await treeView.sortPagesOfParent(parentTitle, desiredOrder);
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

  test('copy-page', async ({ page }) => {
    const title = `Page To Copy ${Date.now()}`;

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

    const copyPageDialog = new CopyPageDialog(page);
    await viewPage.clickCopyPageButton();

    const newTitle = `Copy of ${title}`;
    await copyPageDialog.fillTitle(newTitle);
    await copyPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 2);
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
