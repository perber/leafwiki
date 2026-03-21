import test from '@playwright/test';
import AddPageDialog from '../pages/AddPageDialog';
import CopyPageDialog from '../pages/CopyPageDialog';
import CreatePageByPathDialog from '../pages/CreatePageByPathDialog';
import DeletePageDialog from '../pages/DeletePageDialog';
import EditPage from '../pages/EditPage';
import EditPageMetadataDialog from '../pages/EditPageMetadataDialog';
import LoginPage from '../pages/LoginPage';
import NotFoundPage from '../pages/NotFoundPage';
import TreeView from '../pages/TreeView';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

const currentDir = __dirname;

test.describe('Authenticated', () => {
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

  test('edit-page', async ({ page }) => {
    const title = `Page To Edit ${Date.now()}`;
    const newContent = `This is the new content!  
**Bold Text**  

for the page edited at ${new Date().toISOString()}
`;

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

    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.writeContent(newContent);
    await editPage.savePage();
    await editPage.closeEditor();

    const content = await viewPage.getContent();
    test.expect(content).toContain('This is the new content!');
    test.expect(content).toContain('Bold Text');
  });

  test('unsaved changes-warning', async ({ page }) => {
    const title = `Page With Unsaved Changes ${Date.now()}`;
    const newContent = `This is some unsaved content!  
**Unsaved Bold Text**  

for the page edited at ${new Date().toISOString()}
`;

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
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.writeContent(newContent);

    let dialogType: string | undefined;

    page.once('dialog', (dialog) => {
      dialogType = dialog.type();
      dialog.dismiss().catch(() => {
        // Ignore errors from dismissing the dialog
      });
    });

    let navError: unknown = null;

    try {
      await page.goto('/');
    } catch (e) {
      navError = e;
    }

    test.expect(dialogType).toBe('beforeunload');

    test.expect(String((navError as Error)?.message ?? '')).toMatch(/ERR_ABORTED/);
  });

  test('create-page-with-mermaid', async ({ page }) => {
    const title = `Page With Mermaid ${Date.now()}`;
    const mermaidContent = `\`\`\`mermaid
graph TD;
    A-->B;
\`\`\``;

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

    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.writeContent(mermaidContent);
    await editPage.savePage();
    await editPage.closeEditor();

    // expects at least one SVG element (the mermaid diagram)
    const svgCount = await viewPage.amountOfSVGElements();
    test.expect(svgCount).toBeGreaterThan(0);
  });

  test('create-page-on-not-found-page', async ({ page }) => {
    const slug = `page-from-not-found-${Date.now()}`;
    const pagePath = `/${slug}`;

    const notfoundPage = new NotFoundPage(page);
    await notfoundPage.goto(pagePath);

    test.expect(await notfoundPage.isNotFoundPage()).toBeTruthy();

    await notfoundPage.clickCreatePageButton();
    const createPageByPathDialog = new CreatePageByPathDialog(page);
    await createPageByPathDialog.clickCreate();

    // Check if we are in edit mode
    const editPage = new EditPage(page);
    await editPage.closeEditor();

    // Verify page creation
    const viewPage = new ViewPage(page);
    const pageTitle = await viewPage.getTitle();
    test.expect(pageTitle).toBe(slug);
  });

  // test move
  test('move-page-subpage-to-root-level', async ({ page }) => {
    const parentTitle = `Move Parent Page ${Date.now()}`;
    const childTitle = `Child Page of ${parentTitle}`;

    // Create parent page
    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(parentTitle);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);
    // Create child page
    await treeView.createSubPageOfParent(parentTitle, childTitle);
    await treeView.expectNumberOfTreeNodes(curNodeCount + 2);

    // Move child page to root level
    await treeView.movePageToTopLevel(parentTitle, childTitle);
    // Verify the move
    const nodeRow = page
      .locator('div[data-testid^="tree-node-"]')
      .filter({ hasText: childTitle })
      .first();

    test.expect(await nodeRow.count()).toBe(1);
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

  test('nested-delete-operation', async ({ page }) => {
    const parentTitle = `Delete Parent Page ${Date.now()}`;
    const childTitle = `Child Page ${Date.now()}`;

    // Create parent page
    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(parentTitle);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);
    // Create child page
    await treeView.createSubPageOfParent(parentTitle, childTitle);
    await treeView.expectNumberOfTreeNodes(curNodeCount + 2);

    // Delete parent page
    await treeView.clickPageByTitle(parentTitle);
    const viewPage = new ViewPage(page);
    await viewPage.clickDeletePageButton();

    const deletePageDialog = new DeletePageDialog(page);
    test.expect(await deletePageDialog.dialogTextVisible()).toBeTruthy();
    await deletePageDialog.confirmDeletion();

    // The dialog stays open, because we need to confirm nested deletion
    test.expect(await deletePageDialog.dialogTextVisible()).toBeTruthy();

    await deletePageDialog.confirmNestedDeletion();
    await treeView.expectNumberOfTreeNodes(curNodeCount);
    // Dialog should be closed now
    test.expect(await deletePageDialog.dialogTextVisible()).toBeFalsy();
  });

  // disable this test cases, because it is flaky
  // TODO: fix the flakiness
  /*
  test('search-page', async ({ page }) => {
    const title = `Page To Search ${Date.now()}`;
    const content = `This is the content of the page to search, created at ${new Date().toISOString()}`;

    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(title);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);

    // open edit mode
    await treeView.clickPageByTitle(title);
    const viewPage = new ViewPage(page);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.writeContent(content);
    await editPage.savePage();
    await editPage.closeEditor();

    // switch to search tab
    await viewPage.switchToSearchTab();

    const searchView = new SearchView(page);
    await searchView.enterSearchQuery(title);

    const result = await searchView.searchResultContainsPageTitle(title);
    test.expect(result).toBeTruthy();

    // clear search
    await searchView.clearSearch();
  });
  */

  test('markdown-relative-link-navigates-to-sibling-page', async ({ page }) => {
    const suffix = Date.now();
    const parentTitle = 'markdown-link-parent-' + suffix;
    const sourceTitle = 'source-' + suffix;
    const targetTitle = 'target-' + suffix;
    const linkLabel = 'Go to sibling target';

    const treeView = new TreeView(page);
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(parentTitle);
    await addPageDialog.submitWithoutRedirect();

    await treeView.createSubPageOfParent(parentTitle, sourceTitle);
    await treeView.createSubPageOfParent(parentTitle, targetTitle);
    await treeView.expandNodeByTitle(parentTitle);
    await treeView.clickPageByTitle(sourceTitle);

    const viewPage = new ViewPage(page);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.writeContent('[' + linkLabel + '](../' + targetTitle + ')');
    await editPage.savePage();
    await editPage.closeEditor();

    await page.getByRole('link', { name: linkLabel }).click();
    await page.waitForURL(new RegExp('/' + parentTitle + '/' + targetTitle + '$'));

    await test.expect(page.locator('article>h1')).toHaveText(targetTitle);
  });

  test('delete-current-subpage-redirects-to-parent-page', async ({ page }) => {
    const suffix = Date.now();
    const parentTitle = 'delete-parent-' + suffix;
    const childTitle = 'delete-child-' + suffix;

    const treeView = new TreeView(page);
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(parentTitle);
    await addPageDialog.submitWithoutRedirect();

    await treeView.createSubPageOfParent(parentTitle, childTitle);
    await treeView.expandNodeByTitle(parentTitle);
    await treeView.clickPageByTitle(childTitle);

    const viewPage = new ViewPage(page);
    await viewPage.clickDeletePageButton();

    const deletePageDialog = new DeletePageDialog(page);
    await deletePageDialog.confirmDeletion();
    await page.waitForURL(new RegExp('/' + parentTitle + '$'));

    test.expect(await viewPage.getTitle()).toBe(parentTitle);
  });

  test('delete-unrelated-page-keeps-current-page-open', async ({ page }) => {
    const suffix = Date.now();
    const currentTitle = 'current-page-' + suffix;
    const otherTitle = 'other-page-' + suffix;

    const treeView = new TreeView(page);
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(currentTitle);
    await addPageDialog.submitWithoutRedirect();

    await treeView.clickRootAddButton();
    await addPageDialog.fillTitle(otherTitle);
    await addPageDialog.submitWithoutRedirect();

    await treeView.clickPageByTitle(currentTitle);

    const viewPage = new ViewPage(page);
    test.expect(await viewPage.getTitle()).toBe(currentTitle);

    const nodeRow = page
      .locator('div[data-testid^="tree-node-"]')
      .filter({ hasText: otherTitle })
      .first();

    await nodeRow.scrollIntoViewIfNeeded();
    await nodeRow.hover();

    const moreActionsButton = nodeRow.locator(
      'button[data-testid="tree-view-action-button-open-more-actions"]',
    );
    await moreActionsButton.click({ force: true });

    const deleteButton = page.locator('div[data-testid="tree-view-action-button-delete"]');
    await deleteButton.click({ force: true });

    const deletePageDialog = new DeletePageDialog(page);
    await deletePageDialog.confirmDeletion();

    test.expect(await viewPage.getTitle()).toBe(currentTitle);
    await page.waitForURL(new RegExp('/' + currentTitle + '$'));
  });

  test('cannot-delete-current-page-while-editing-it', async ({ page }) => {
    const title = 'Editing Delete Guard ' + Date.now();
    const warningText =
      'This page is currently being edited. Please close the editor before deleting it.';

    const treeView = new TreeView(page);
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(title);
    await addPageDialog.submitWithoutRedirect();

    await treeView.clickPageByTitle(title);

    const viewPage = new ViewPage(page);
    await viewPage.clickEditPageButton();

    const nodeRow = page
      .locator('div[data-testid^="tree-node-"]')
      .filter({ hasText: title })
      .first();

    await nodeRow.scrollIntoViewIfNeeded();
    await nodeRow.hover();

    const moreActionsButton = nodeRow.locator(
      'button[data-testid="tree-view-action-button-open-more-actions"]',
    );
    await moreActionsButton.click({ force: true });

    const deleteButton = page.locator('div[data-testid="tree-view-action-button-delete"]');
    await deleteButton.click({ force: true });

    const deletePageDialog = new DeletePageDialog(page);
    test.expect(await deletePageDialog.dialogTextVisible()).toBeFalsy();
    await page.getByText(warningText).waitFor({ state: 'visible' });
    test.expect(await page.locator('.cm-editor').isVisible()).toBeTruthy();
  });

  test('edit-metadata-on-nested-page-keeps-parent-path', async ({ page }) => {
    const suffix = Date.now();
    const parentTitle = 'meta-parent-' + suffix;
    const childTitle = 'meta-child-' + suffix;
    const renamedChildTitle = 'meta-child-renamed-' + suffix;
    const expectedPath = parentTitle + '/' + renamedChildTitle;

    const treeView = new TreeView(page);
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(parentTitle);
    await addPageDialog.submitWithoutRedirect();

    await treeView.createSubPageOfParent(parentTitle, childTitle);
    await treeView.expandNodeByTitle(parentTitle);
    await treeView.clickPageByTitle(childTitle);

    const viewPage = new ViewPage(page);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.openMetadataDialog();

    const metadataDialog = new EditPageMetadataDialog(page);
    await metadataDialog.fillTitle(renamedChildTitle);
    await metadataDialog.expectSlug(renamedChildTitle);
    await metadataDialog.expectPath(expectedPath);
    await metadataDialog.submit();

    await editPage.savePage();
    await editPage.closeEditor();

    await page.waitForURL(new RegExp('/' + expectedPath + '$'));
  });

  test('test-asset-upload-and-use-in-page', async ({ page }) => {
    const title = `Page With Asset ${Date.now()}`;
    // const assetFileName = 'test-image.png';
    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();
    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(title);
    await addPageDialog.submitWithoutRedirect();
    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);
    await treeView.clickPageByTitle(title);
    let viewPage = new ViewPage(page);
    const pageTitle = await viewPage.getTitle();
    test.expect(pageTitle).toBe(title);
    await viewPage.clickEditPageButton();
    // pause to see the editor
    const editPage = new EditPage(page);
    // Opens asset manager in edit mode
    editPage.openAssetManager();
    // Upload asset
    await editPage.uploadAsset(currentDir + '/../assets/upload-test.png');
    await editPage.listAmountOfAssets().then((count) => {
      test.expect(count).toBeGreaterThan(0);
    });
    // Insert first asset into page
    await editPage.insertFirstAssetIntoPage();
    await editPage.savePage();
    await editPage.closeEditor();
    viewPage = new ViewPage(page);
    await viewPage.amountOfImages().then((count) => {
      test.expect(count).toBeGreaterThan(0);
    });
  });
});
