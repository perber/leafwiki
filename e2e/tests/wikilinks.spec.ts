import test, { Page, expect } from '@playwright/test';
import EditPage from '../pages/EditPage';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

// ─── API helpers ─────────────────────────────────────────────────────────────

function getCsrfScript(): string {
  return `
    const hostMatch =
      document.cookie.match(/(?:^|;\\s*)__Host-leafwiki_csrf=([^;]+)/) ??
      document.cookie.match(/(?:^|;\\s*)leafwiki_csrf=([^;]+)/);
    if (!hostMatch) throw new Error('Missing CSRF token cookie');
    try { return decodeURIComponent(hostMatch[1]); } catch { return hostMatch[1]; }
  `;
}

async function createPage(
  page: Page,
  input: { title: string; slug: string; content?: string },
) {
  await page.evaluate(
    async ({ title, slug, content, csrfScript }) => {
      const csrfToken = new Function(csrfScript)() as string;

      const createRes = await fetch('/api/pages', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
        body: JSON.stringify({ parentId: null, title, slug, kind: 'page' }),
      });
      if (!createRes.ok) throw new Error(`create failed: ${createRes.status}`);

      if (content) {
        const created = (await createRes.json()) as { id: string; version: string };
        const updateRes = await fetch(`/api/pages/${created.id}`, {
          method: 'PUT',
          credentials: 'include',
          headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
          body: JSON.stringify({ version: created.version, title, slug, content, tags: [], properties: {} }),
        });
        if (!updateRes.ok) throw new Error(`update failed: ${updateRes.status}`);
      }
    },
    { ...input, csrfScript: getCsrfScript() },
  );
}

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
    await editPage.writeContent(`[[FilterableWikiPage`);

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
    await editPage.writeContent(`[[InsertTargetPage`);

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
    await editPage.writeContent(`[[PreviewLinkTarget`);

    await editPage.waitForAutocompleteDropdown();
    await editPage.selectAutocompleteOption(targetTitle);

    await editPage.savePage();
    await editPage.closeEditor();

    // WikiLink should render as a hyperlink in the page view
    await expect(
      page.locator('#markdown-preview-container a', { hasText: targetTitle }),
    ).toBeVisible();
  });
});
