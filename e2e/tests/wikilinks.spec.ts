import test, { expect } from '@playwright/test';
import { createPage } from '../helpers/api';
import CreatePageByPathDialog from '../pages/CreatePageByPathDialog';
import EditPage from '../pages/EditPage';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

// ─── Tests ────────────────────────────────────────────────────────────────────

test.describe('WikiLink autocomplete', () => {
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

  test('wikilink-autocomplete-shows-popup-on-double-bracket', async ({ page }) => {
    const stamp = Date.now();
    const targetSlug = `wikilink-target-${stamp}`;
    const editorSlug = `wikilink-editor-${stamp}`;

    await createPage(page, { title: `WikiLink Target ${stamp}`, slug: targetSlug });
    await createPage(page, { title: `WikiLink Editor ${stamp}`, slug: editorSlug });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${editorSlug}`);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.writeContent('[[');

    await editPage.waitForAutocompleteDropdown();
    await expect(page.locator('.cm-tooltip-autocomplete')).toBeVisible();
  });

  test('wikilink-autocomplete-filters-by-typed-title', async ({ page }) => {
    const stamp = Date.now();
    const targetSlug = `wikilink-filter-target-${stamp}`;
    const editorSlug = `wikilink-filter-editor-${stamp}`;
    const targetTitle = `FilterableWikiPage ${stamp}`;

    await createPage(page, { title: targetTitle, slug: targetSlug });
    await createPage(page, { title: `WikiLink Filter Editor ${stamp}`, slug: editorSlug });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${editorSlug}`);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.writeContent('[[FilterableWikiPage');

    await editPage.waitForAutocompleteDropdown();

    await expect(
      page.locator('.cm-tooltip-autocomplete .cm-completionLabel', { hasText: targetTitle }),
    ).toBeVisible();
  });

  test('wikilink-autocomplete-inserts-wikilink-on-selection', async ({ page }) => {
    const stamp = Date.now();
    const targetSlug = `wikilink-insert-target-${stamp}`;
    const editorSlug = `wikilink-insert-editor-${stamp}`;
    const targetTitle = `InsertTargetPage ${stamp}`;

    await createPage(page, { title: targetTitle, slug: targetSlug });
    await createPage(page, { title: `WikiLink Insert Editor ${stamp}`, slug: editorSlug });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${editorSlug}`);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.writeContent('[[InsertTargetPage');

    await editPage.waitForAutocompleteDropdown();
    await editPage.selectAutocompleteOption(targetTitle);

    const content = await editPage.getEditorContent();
    expect(content).toContain(`[[${targetTitle}]]`);
    // No double closing brackets
    expect(content).not.toContain(`[[${targetTitle}]]]]`);
  });

  test('wikilink-autocomplete-renders-as-link-in-preview-after-save', async ({ page }) => {
    const stamp = Date.now();
    const targetSlug = `wikilink-preview-target-${stamp}`;
    const editorSlug = `wikilink-preview-editor-${stamp}`;
    const targetTitle = `PreviewLinkTarget ${stamp}`;

    await createPage(page, { title: targetTitle, slug: targetSlug });
    await createPage(page, { title: `WikiLink Preview Editor ${stamp}`, slug: editorSlug });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${editorSlug}`);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.writeContent('[[PreviewLinkTarget');

    await editPage.waitForAutocompleteDropdown();
    await editPage.selectAutocompleteOption(targetTitle);

    await editPage.savePage();
    await editPage.closeEditor();

    // WikiLink should render as a hyperlink in the page view
    await expect(page.locator('article a', { hasText: targetTitle })).toBeVisible();
  });

  test('missing-wikilink-can-create-page-from-preview', async ({ page }) => {
    const stamp = Date.now();
    const editorSlug = `wikilink-missing-editor-${stamp}`;
    const missingTitle = `Missing WikiLink Target ${stamp}`;

    await createPage(page, {
      title: `WikiLink Missing Editor ${stamp}`,
      slug: editorSlug,
      content: `[[${missingTitle}]]`,
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${editorSlug}`);

    const missingLinkButton = page.getByRole('button', { name: missingTitle });
    await expect(missingLinkButton).toBeVisible();
    await missingLinkButton.click();

    await expect(page.getByTestId('create-page-by-path-title-input')).toHaveValue(missingTitle);

    const createPageByPathDialog = new CreatePageByPathDialog(page);
    await createPageByPathDialog.clickCreate();

    const editPage = new EditPage(page);
    await editPage.closeEditor();

    await expect(page.locator('article > h1')).toHaveText(missingTitle);
    await expect(page.locator('article a', { hasText: missingTitle })).toBeVisible();
  });

  test('ambiguous-wikilink-opens-disambiguation-dialog', async ({ page }) => {
    const stamp = Date.now();
    const duplicateTitle = `Ambiguous WikiLink Target ${stamp}`;
    const firstSlug = `wikilink-ambiguous-first-${stamp}`;
    const secondSlug = `wikilink-ambiguous-second-${stamp}`;
    const editorSlug = `wikilink-ambiguous-editor-${stamp}`;

    await createPage(page, { title: duplicateTitle, slug: firstSlug });
    await createPage(page, { title: duplicateTitle, slug: secondSlug });
    await createPage(page, {
      title: `WikiLink Ambiguous Editor ${stamp}`,
      slug: editorSlug,
      content: `[[${duplicateTitle}]]`,
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${editorSlug}`);

    const ambiguousLinkButton = page.getByRole('button', { name: duplicateTitle });
    await expect(ambiguousLinkButton).toBeVisible();
    await ambiguousLinkButton.click();

    const dialog = page.getByTestId('wikilink-disambiguation-dialog');
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText(`/${firstSlug}`)).toBeVisible();
    await expect(dialog.getByText(`/${secondSlug}`)).toBeVisible();

    await dialog.getByText(`/${firstSlug}`).click();

    await expect(page).toHaveURL(new RegExp(`/${firstSlug}$`));
    await expect(page.locator('article > h1')).toHaveText(duplicateTitle);
  });
});
