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

async function createPageWithMetadata(
  page: Page,
  input: {
    title: string;
    slug: string;
    content: string;
    tags?: string[];
    properties?: Record<string, string>;
  },
) {
  await page.evaluate(
    async ({ title, slug, content, tags, properties, csrfScript }) => {
      const csrfToken = new Function(csrfScript)() as string;

      const createRes = await fetch('/api/pages', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
        body: JSON.stringify({ parentId: null, title, slug, kind: 'page' }),
      });
      if (!createRes.ok) throw new Error(`create failed: ${createRes.status}`);

      const created = (await createRes.json()) as { id: string; version: string };

      const updateRes = await fetch(`/api/pages/${created.id}`, {
        method: 'PUT',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
        body: JSON.stringify({
          version: created.version,
          title,
          slug,
          content,
          tags: tags ?? [],
          properties: properties ?? {},
        }),
      });
      if (!updateRes.ok) throw new Error(`update failed: ${updateRes.status}`);
    },
    { ...input, csrfScript: getCsrfScript() },
  );
}

async function updatePageByPath(page: Page, input: { path: string; content: string }) {
  await page.evaluate(
    async ({ path, content, csrfScript }) => {
      const csrfToken = new Function(csrfScript)() as string;

      const pageRes = await fetch(
        `/api/pages/by-path?path=${encodeURIComponent(path.replace(/^\/+/, ''))}`,
        { credentials: 'include', headers: { 'X-CSRF-Token': csrfToken } },
      );
      if (!pageRes.ok) throw new Error(`load failed: ${pageRes.status}`);
      const current = (await pageRes.json()) as {
        id: string;
        title: string;
        slug: string;
        version: string;
      };

      const updateRes = await fetch(`/api/pages/${current.id}`, {
        method: 'PUT',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
        body: JSON.stringify({
          version: current.version,
          title: current.title,
          slug: current.slug,
          content,
        }),
      });
      if (!updateRes.ok) throw new Error(`update failed: ${updateRes.status}`);
    },
    { ...input, csrfScript: getCsrfScript() },
  );
}

// ─── Panel helpers ────────────────────────────────────────────────────────────

async function openFrontmatterPanel(page: Page) {
  const trigger = page.locator('.page-frontmatter-panel__trigger');
  await trigger.waitFor({ state: 'visible' });
  await trigger.click();
  await page.locator('[data-testid="page-frontmatter-tag-input"]').waitFor({ state: 'visible' });
}

async function addTag(page: Page, tag: string) {
  const input = page.locator('[data-testid="page-frontmatter-tag-input"]');
  await input.fill(tag);
  await page.keyboard.press('Enter');
}

async function removeTag(page: Page, tag: string) {
  await page
    .locator('.page-frontmatter-panel__chip')
    .filter({ hasText: tag })
    .locator('.page-frontmatter-panel__chip-remove')
    .click();
}

async function addProperty(page: Page, key: string, value: string) {
  await page.locator('[data-testid="page-frontmatter-add-field"]').click();
  const fields = page.locator('[data-testid^="page-frontmatter-field-key-"]');
  const count = await fields.count();
  const lastIndex = count - 1;
  await page.locator(`[data-testid="page-frontmatter-field-key-${lastIndex}"]`).fill(key);
  await page.locator(`[data-testid="page-frontmatter-field-value-${lastIndex}"]`).fill(value);
}

// ─── Tests ────────────────────────────────────────────────────────────────────

test.describe('Editor', () => {
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

  // ── Loading ────────────────────────────────────────────────────────────────

  test('editor-loads-tags-and-properties-from-api', async ({ page }) => {
    const stamp = Date.now();
    const slug = `editor-load-meta-${stamp}`;

    await createPageWithMetadata(page, {
      title: `Editor Load Meta ${stamp}`,
      slug,
      content: 'Page with metadata.',
      tags: ['alpha', 'beta'],
      properties: { owner: 'alice', priority: 'high' },
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.clickEditPageButton();

    await openFrontmatterPanel(page);

    // Tags are shown as chips
    await expect(
      page.locator('.page-frontmatter-panel__chip').filter({ hasText: 'alpha' }),
    ).toBeVisible();
    await expect(
      page.locator('.page-frontmatter-panel__chip').filter({ hasText: 'beta' }),
    ).toBeVisible();

    // Properties are shown in the field inputs
    const keyInputs = page.locator('[data-testid^="page-frontmatter-field-key-"]');
    const valueInputs = page.locator('[data-testid^="page-frontmatter-field-value-"]');
    await expect(keyInputs).toHaveCount(2);

    const keys = await keyInputs.evaluateAll((els: HTMLInputElement[]) => els.map((e) => e.value));
    const values = await valueInputs.evaluateAll((els: HTMLInputElement[]) =>
      els.map((e) => e.value),
    );
    expect(keys).toContain('owner');
    expect(keys).toContain('priority');
    expect(values).toContain('alice');
    expect(values).toContain('high');
  });

  // ── Saving ─────────────────────────────────────────────────────────────────

  test('editor-saves-tags-and-properties', async ({ page }) => {
    const stamp = Date.now();
    const slug = `editor-save-meta-${stamp}`;

    await createPageWithMetadata(page, {
      title: `Editor Save Meta ${stamp}`,
      slug,
      content: 'Page content.',
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.clickEditPageButton();

    await openFrontmatterPanel(page);
    await addTag(page, 'release');
    await addProperty(page, 'status', 'draft');

    const editPage = new EditPage(page);
    await editPage.savePage();
    await editPage.closeEditor();

    // Viewer shows the tag chip
    await page
      .locator('.page-metadata__tag-chip')
      .filter({ hasText: 'release' })
      .waitFor({ state: 'visible' });

    // Properties section exists; expand it and verify the value
    const propsToggle = page.locator('.page-metadata__props-toggle');
    await propsToggle.waitFor({ state: 'visible' });
    await propsToggle.click();
    await expect(
      page.locator('.page-metadata__prop-key').filter({ hasText: 'status' }),
    ).toBeVisible();
    await expect(
      page.locator('.page-metadata__prop-value').filter({ hasText: 'draft' }),
    ).toBeVisible();
  });

  test('editor-removes-tags-when-all-tags-are-cleared', async ({ page }) => {
    const stamp = Date.now();
    const slug = `editor-remove-tags-${stamp}`;

    await createPageWithMetadata(page, {
      title: `Editor Remove Tags ${stamp}`,
      slug,
      content: 'Page with tags.',
      tags: ['leafwiki', 'andere'],
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.clickEditPageButton();

    await openFrontmatterPanel(page);
    await removeTag(page, 'leafwiki');
    await removeTag(page, 'andere');

    const editPage = new EditPage(page);
    await editPage.savePage();
    await editPage.closeEditor();

    await expect(page.locator('.page-metadata__tag-chip')).toHaveCount(0);

    const fetchedTags = await page.evaluate(
      async ({ path, csrfScript }) => {
        const csrfToken = new Function(csrfScript)() as string;
        const response = await fetch(`/api/pages/by-path?path=${encodeURIComponent(path)}`, {
          credentials: 'include',
          headers: { 'X-CSRF-Token': csrfToken },
        });
        if (!response.ok) {
          throw new Error(`load failed: ${response.status}`);
        }
        const data = (await response.json()) as { tags?: string[] };
        return data.tags ?? [];
      },
      { path: slug, csrfScript: getCsrfScript() },
    );

    expect(fetchedTags).toEqual([]);
  });

  test('editor-updates-tags-and-properties-on-re-save', async ({ page }) => {
    const stamp = Date.now();
    const slug = `editor-update-meta-${stamp}`;

    await createPageWithMetadata(page, {
      title: `Editor Update Meta ${stamp}`,
      slug,
      content: 'Some content.',
      tags: ['initial'],
      properties: { phase: 'one' },
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.clickEditPageButton();

    await openFrontmatterPanel(page);

    // Remove the 'initial' tag
    await page
      .locator('.page-frontmatter-panel__chip')
      .filter({ hasText: 'initial' })
      .locator('button[aria-label*="Remove tag"]')
      .click();

    await addTag(page, 'updated');

    // Update the property value
    await page.locator('[data-testid="page-frontmatter-field-value-0"]').fill('two');

    const editPage = new EditPage(page);
    await editPage.savePage();
    await editPage.closeEditor();

    // Viewer shows updated tag, not the old one
    await expect(
      page.locator('.page-metadata__tag-chip').filter({ hasText: 'updated' }),
    ).toBeVisible();
    await expect(
      page.locator('.page-metadata__tag-chip').filter({ hasText: 'initial' }),
    ).not.toBeVisible();

    // Updated property value
    await page.locator('.page-metadata__props-toggle').click();
    await expect(
      page.locator('.page-metadata__prop-value').filter({ hasText: 'two' }),
    ).toBeVisible();
  });

  // ── Dirty state ─────────────────────────────────────────────────────────────

  test('editor-save-button-disabled-when-page-is-clean', async ({ page }) => {
    const stamp = Date.now();
    const slug = `editor-clean-state-${stamp}`;

    await createPageWithMetadata(page, {
      title: `Editor Clean State ${stamp}`,
      slug,
      content: 'Unmodified content.',
      tags: ['existing-tag'],
      properties: { key: 'value' },
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.clickEditPageButton();

    // Without making changes, the save button must be disabled
    const saveButton = page.locator('button[data-testid="save-page-button"]');
    await saveButton.waitFor({ state: 'visible' });
    await expect(saveButton).toBeDisabled();
  });

  test('editor-save-button-enabled-after-tag-change', async ({ page }) => {
    const stamp = Date.now();
    const slug = `editor-dirty-tag-${stamp}`;

    await createPageWithMetadata(page, {
      title: `Editor Dirty Tag ${stamp}`,
      slug,
      content: 'Content.',
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.clickEditPageButton();

    const saveButton = page.locator('button[data-testid="save-page-button"]');
    await saveButton.waitFor({ state: 'visible' });
    await expect(saveButton).toBeDisabled();

    await openFrontmatterPanel(page);
    await addTag(page, 'new-tag');

    await expect(saveButton).toBeEnabled();
  });

  // ── Error codes ─────────────────────────────────────────────────────────────

  test('editor-version-conflict-error-code-shown-on-save', async ({ page }) => {
    const stamp = Date.now();
    const slug = `editor-conflict-${stamp}`;

    await createPageWithMetadata(page, {
      title: `Editor Conflict ${stamp}`,
      slug,
      content: 'Original content.',
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.writeContent(' local change');

    // Simulate a concurrent save from another session
    await updatePageByPath(page, {
      path: `/${slug}`,
      content: 'Concurrent save from another session.',
    });

    await page.locator('button[data-testid="save-page-button"]').click();

    // The version-conflict toast must appear (identified by its error-code test ID)
    await page.getByTestId('page-save-version-conflict-toast').waitFor({ state: 'visible' });

    // Accepting the conflict resolves successfully
    await page.getByTestId('page-save-version-conflict-action').click();
    await page.getByText('Page saved successfully').last().waitFor({ state: 'visible' });
  });

  test('editor-validation-error-blocks-save-with-reserved-property-key', async ({ page }) => {
    const stamp = Date.now();
    const slug = `editor-validation-${stamp}`;

    await createPageWithMetadata(page, {
      title: `Editor Validation ${stamp}`,
      slug,
      content: 'Content.',
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.clickEditPageButton();

    await openFrontmatterPanel(page);
    // "tags" is a reserved property key — the frontend must reject this before sending the request
    await addProperty(page, 'tags', 'some-value');

    await page.locator('button[data-testid="save-page-button"]').click();

    // The per-field error must be visible and the page must not be saved
    const keyError = page.locator('[data-testid="page-frontmatter-field-key-error-0"]');
    await keyError.waitFor({ state: 'visible' });
    await expect(keyError).toContainText('reserved');

    // No success toast
    await expect(page.getByText('Page saved successfully')).not.toBeVisible();
  });

  test('editor-validation-error-blocks-save-with-empty-property-key', async ({ page }) => {
    const stamp = Date.now();
    const slug = `editor-empty-key-${stamp}`;

    await createPageWithMetadata(page, {
      title: `Editor Empty Key ${stamp}`,
      slug,
      content: 'Content.',
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.clickEditPageButton();

    await openFrontmatterPanel(page);

    // Add a field but leave the key empty — only fill in a value
    await page.locator('[data-testid="page-frontmatter-add-field"]').click();
    await page.locator('[data-testid="page-frontmatter-field-value-0"]').fill('some-value');

    await page.locator('button[data-testid="save-page-button"]').click();

    const keyError = page.locator('[data-testid="page-frontmatter-field-key-error-0"]');
    await keyError.waitFor({ state: 'visible' });
    await expect(keyError).toContainText('empty');

    // No success toast
    await expect(page.getByText('Page saved successfully')).not.toBeVisible();
  });
});
