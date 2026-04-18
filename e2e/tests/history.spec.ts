import test, { expect } from '@playwright/test';
import AddPageDialog from '../pages/AddPageDialog';
import EditPage from '../pages/EditPage';
import EditPageMetadataDialog from '../pages/EditPageMetadataDialog';
import LoginPage from '../pages/LoginPage';
import TreeView from '../pages/TreeView';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

// Helper: create a page with multiple revisions so history tests have data.
async function createPageWithRevisions(
  page: import('@playwright/test').Page,
  title: string,
  revisionContents: string[],
) {
  const treeView = new TreeView(page);
  await treeView.clickRootAddButton();

  const addPageDialog = new AddPageDialog(page);
  await addPageDialog.fillTitle(title);
  await addPageDialog.submitWithoutRedirect();

  await treeView.clickPageByTitle(title);

  const editPage = new EditPage(page);
  const viewPage = new ViewPage(page);

  for (const content of revisionContents) {
    await viewPage.clickEditPageButton();
    await editPage.writeContent(content);
    await editPage.savePage();
    await editPage.closeEditor();
  }

  await treeView.clickPageByTitle(title);
  await expect(page.locator('article > h1')).toHaveText(title);

  return viewPage;
}

test.describe('History', () => {
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

  test('revision-list-panel-visible-on-history-page', async ({ page }) => {
    const title = `History List Panel ${Date.now()}`;
    const viewPage = await createPageWithRevisions(page, title, [
      'First revision content',
      '\nSecond revision content',
    ]);

    await viewPage.openCurrentPageHistory();
    await viewPage.expectRevisionListVisible();
    await expect(page.locator('[data-testid^="history-sidebar-revision-"]').first()).toBeVisible();
  });

  test('current-revision-is-visible-and-badged', async ({ page }) => {
    const title = `History Current Badge ${Date.now()}`;
    const viewPage = await createPageWithRevisions(page, title, [
      'First revision content',
      '\nSecond revision content',
    ]);

    await viewPage.openCurrentPageHistory();
    await viewPage.expectRevisionListVisible();

    const currentBadge = page.locator('[data-testid^="history-sidebar-revision-current-badge-"]');
    await expect(currentBadge).toHaveCount(1);
    await expect(currentBadge).toHaveText('Active version');

    const currentRevision = currentBadge.locator('xpath=ancestor::button[1]');
    await currentRevision.click();
    await expect(page.getByTestId('page-history-page-restore')).toBeDisabled();
  });

  test('revision-list-stays-visible-after-selecting-revision', async ({ page }) => {
    // Regression for: revision list disappearing when a revision is opened.
    const title = `History Stays Visible ${Date.now()}`;
    const viewPage = await createPageWithRevisions(page, title, [
      'First content',
      '\nSecond content',
    ]);

    await viewPage.openCurrentPageHistory();
    await viewPage.expectRevisionListVisible();

    // Select the first revision — the list must still be visible afterwards.
    await viewPage.openRevisionAt(0);

    await viewPage.expectRevisionListVisible();
    await expect(page.locator('[data-testid^="history-sidebar-revision-"]').first()).toBeVisible();
    await expect(page.getByTestId('page-history-page-content')).toBeVisible();
  });

  test('preview-tab-is-active-by-default', async ({ page }) => {
    // Regression for: "Changes" was the default tab — "Preview" should be first and active.
    const title = `History Preview Default ${Date.now()}`;
    const viewPage = await createPageWithRevisions(page, title, [
      'Content for preview test',
      '\nSecond preview revision',
    ]);

    await viewPage.openCurrentPageHistory();
    await viewPage.openRevisionAt(0);

    const previewTab = page.locator('[data-testid="page-history-page-preview-tab"]');
    await previewTab.waitFor({ state: 'visible' });

    // Preview tab must be active without any user interaction.
    await expect(previewTab).toHaveClass(/page-history__tab-button--active/);
    await expect(page.getByTestId('page-history-page-content')).toBeVisible();
  });

  test('diff-section-references-active-version', async ({ page }) => {
    // The diff heading should tell the user what they are comparing against.
    const title = `History Diff Label ${Date.now()}`;
    const viewPage = await createPageWithRevisions(page, title, [
      'Original content',
      '\nUpdated content',
    ]);

    await viewPage.openCurrentPageHistory();
    await viewPage.openRevisionAt(0);

    // Switch to the Changes tab.
    await page.locator('[data-testid="page-history-page-changes-tab"]').click();
    await page.getByTestId('page-history-page-content').waitFor({ state: 'visible' });

    await expect(page.getByTestId('page-history-page-content')).toContainText(
      'compared to the active version',
    );
  });

  test('active-revision-shows-no-diff', async ({ page }) => {
    const title = `History Active Diff ${Date.now()}`;
    const viewPage = await createPageWithRevisions(page, title, [
      'First revision content',
      '\nSecond revision content',
    ]);

    await viewPage.openCurrentPageHistory();

    const currentBadge = page.locator('[data-testid^="history-sidebar-revision-current-badge-"]');
    const currentRevision = currentBadge.locator('xpath=ancestor::button[1]');
    await currentRevision.click();

    await page.locator('[data-testid="page-history-page-changes-tab"]').click();
    await expect(page.getByTestId('page-history-page-content')).toContainText(
      'No differences from the current version.',
    );
  });

  test('structure-changes-are-visible-in-history-header', async ({ page }) => {
    const suffix = Date.now();
    const originalTitle = `History Structure ${suffix}`;
    const renamedTitle = `History Structure Renamed ${suffix}`;
    const renamedSlug = `history-structure-renamed-${suffix}`;
    const viewPage = await createPageWithRevisions(page, originalTitle, [
      'First revision content',
      '\nSecond revision content',
    ]);

    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.openMetadataDialog();

    const metadataDialog = new EditPageMetadataDialog(page);
    await metadataDialog.fillTitle(renamedTitle);
    await metadataDialog.fillSlug(renamedSlug);
    await metadataDialog.submit();

    await editPage.closeEditor();
    await viewPage.openCurrentPageHistory();
    await page.locator('[data-testid="page-history-page-changes-tab"]').click();

    const structureChanges = page.getByTestId('page-history-page-structure-changes');
    await expect(structureChanges).toContainText('Title');
    await expect(structureChanges).toContainText(originalTitle);
    await expect(structureChanges).toContainText(renamedTitle);
    await expect(structureChanges).toContainText('Slug');
    await expect(structureChanges).toContainText(`history-structure-${suffix}`);
    await expect(structureChanges).toContainText(renamedSlug);
  });

  test('selecting-a-revision-keeps-current-history-route-after-rename', async ({ page }) => {
    const suffix = Date.now();
    const originalTitle = `History Route ${suffix}`;
    const renamedTitle = `history-route-renamed-${suffix}`;
    const viewPage = await createPageWithRevisions(page, originalTitle, [
      'First revision content',
      '\nSecond revision content',
    ]);

    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.openMetadataDialog();

    const metadataDialog = new EditPageMetadataDialog(page);
    await metadataDialog.fillTitle(renamedTitle);
    await metadataDialog.expectSlug(renamedTitle);
    await metadataDialog.submit();

    await editPage.closeEditor();

    await viewPage.openCurrentPageHistory();

    const historyPathBeforeSelection = new URL(page.url()).pathname;

    await viewPage.openRevisionAt(0);

    await expect.poll(() => new URL(page.url()).pathname).toBe(historyPathBeforeSelection);
    await expect(page.getByTestId('page-history-page-content')).toBeVisible();
  });

  test('revision-title-shows-timestamp-not-type-label', async ({ page }) => {
    // Revision list items should show a formatted timestamp, not generic
    // type labels like "Content changed" or "Assets changed".
    const title = `History Timestamp Title ${Date.now()}`;
    const viewPage = await createPageWithRevisions(page, title, [
      'Content to trigger a revision',
      '\nSecond revision to keep one visible in the list',
    ]);

    await viewPage.openCurrentPageHistory();
    await viewPage.expectRevisionListVisible();

    const firstItem = page.locator('[data-testid^="history-sidebar-revision-"]').first();
    await firstItem.waitFor({ state: 'visible' });

    const itemTitle = firstItem.locator('.history-sidebar__item-title');
    await expect(itemTitle).not.toContainText('Content changed');
    await expect(itemTitle).not.toContainText('Assets changed');
    await expect(itemTitle).not.toContainText('Structure updated');
    // A formatted timestamp contains at least a digit (year, day, or time).
    await expect(itemTitle).toContainText(/\d/);
  });

  test('tree-visible-in-sidebar-on-history-page', async ({ page }) => {
    // Regression for: tree sidebar tab not visible when on the history page.
    const title = `History Tree Sidebar ${Date.now()}`;
    const viewPage = await createPageWithRevisions(page, title, ['Some content']);

    await viewPage.openCurrentPageHistory();

    // The explorer (tree) tab must be accessible from the sidebar while in history mode.
    const treeTabButton = page.locator('button[data-testid="sidebar-tree-tab-button"]');
    await treeTabButton.waitFor({ state: 'visible' });
    await treeTabButton.click();

    await expect(page.locator('a[data-testid^="tree-node-link-"]').first()).toBeVisible();
  });

  test('sidebar-tree-visible-in-settings', async ({ page }) => {
    // Regression for: tree sidebar tab not visible on settings pages.
    await page.goto('/settings/branding');
    await page.waitForLoadState('networkidle');

    const treeTabButton = page.locator('button[data-testid="sidebar-tree-tab-button"]');
    await treeTabButton.waitFor({ state: 'visible' });
    await expect(treeTabButton).toBeVisible();
  });

  test('restore-revision', async ({ page }) => {
    const originalContent = `Original ${Date.now()}`;
    const updatedContent = `Updated ${Date.now()}`;
    const title = `history-restore-${Date.now()}`;
    const renamedTitle = `history-restore-renamed-${Date.now()}`;
    const viewPage = await createPageWithRevisions(page, title, [
      originalContent,
      `\n${updatedContent}`,
    ]);

    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.openMetadataDialog();

    const metadataDialog = new EditPageMetadataDialog(page);
    await metadataDialog.fillTitle(renamedTitle);
    await metadataDialog.expectSlug(renamedTitle);
    await metadataDialog.submit();
    await editPage.closeEditor();

    await viewPage.openCurrentPageHistory();
    await viewPage.openRevisionAt(0);

    const restoreButton = page.locator('[data-testid="page-history-page-restore"]');
    await restoreButton.waitFor({ state: 'visible' });
    await restoreButton.click();

    // After restore the history page should reload and show the restored state.
    await page.getByTestId('page-history-page-content').waitFor({ state: 'visible' });
    await expect(page.locator('[data-testid^="history-sidebar-revision-"]').first()).toBeVisible();
    await expect(page.locator('article > h1')).toHaveText(title);
    await expect.poll(() => new URL(page.url()).pathname).toContain(`/history/${title}`);
  });
});
