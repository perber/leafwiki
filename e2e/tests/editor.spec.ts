import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test, { Page, expect } from '@playwright/test';
import { getCsrfScript } from '../helpers/api';
import EditPage from '../pages/EditPage';
import EditPageMetadataDialog from '../pages/EditPageMetadataDialog';
import LoginPage from '../pages/LoginPage';
import TreeView from '../pages/TreeView';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';
const currentDir = __dirname;
const editorPreviewScrollFixturePath = join(
  currentDir,
  '..',
  'assets',
  'editor-preview-scroll-fixture.md',
);

// ─── API helpers ─────────────────────────────────────────────────────────────

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

async function removeProperty(page: Page, index: number) {
  await page.locator('.page-frontmatter-panel__field-remove').nth(index).click();
}

async function clickEditorLineByText(page: Page, text: string) {
  await expect
    .poll(
      () =>
        page.evaluate((lineText) => {
          const scroller = document.querySelector('.cm-scroller');
          if (!(scroller instanceof HTMLElement)) {
            throw new Error('Missing CodeMirror scroller');
          }

          const visibleLine = Array.from(scroller.querySelectorAll('.cm-line')).find((element) =>
            element.textContent?.includes(lineText),
          );
          if (visibleLine instanceof HTMLElement) {
            visibleLine.scrollIntoView({ block: 'center' });
            return 'found';
          }

          const maxScrollTop = Math.max(0, scroller.scrollHeight - scroller.clientHeight);
          if (scroller.scrollTop >= maxScrollTop) {
            return 'not-found';
          }

          scroller.scrollTop = Math.min(
            maxScrollTop,
            scroller.scrollTop + Math.max(scroller.clientHeight * 0.8, 200),
          );
          return 'searching';
        }, text),
      { message: `Expected editor line containing "${text}" to become visible` },
    )
    .toBe('found');

  const targetLine = page.locator('.cm-line').filter({ hasText: text }).first();

  await targetLine.click();
}

async function getPreviewScrollTop(page: Page) {
  return page.locator('#markdown-preview-container').evaluate((element) => {
    if (!(element instanceof HTMLElement)) {
      throw new Error('Expected markdown preview container');
    }

    return element.scrollTop;
  });
}

async function expectPreviewHeadingVisible(page: Page, headingText: string) {
  await expect
    .poll(
      () =>
        page.evaluate(
          ({ text }) => {
            const preview = document.getElementById('markdown-preview-container');
            if (!(preview instanceof HTMLElement)) {
              throw new Error('Expected markdown preview container');
            }

            const headings = Array.from(preview.querySelectorAll('h1, h2, h3, h4, h5, h6'));
            const heading = headings.find((element) => element.textContent?.includes(text)) as
              | HTMLElement
              | undefined;

            if (!heading) return false;

            const previewRect = preview.getBoundingClientRect();
            const headingRect = heading.getBoundingClientRect();
            const top = headingRect.top - previewRect.top;
            const bottom = headingRect.bottom - previewRect.top;
            return top >= 0 && bottom <= preview.clientHeight;
          },
          { text: headingText },
        ),
      { message: `Expected preview heading "${headingText}" to be visible in preview` },
    )
    .toBe(true);

  const bounds = await page.evaluate(
    ({ text }) => {
      const preview = document.getElementById('markdown-preview-container');
      if (!(preview instanceof HTMLElement)) {
        throw new Error('Expected markdown preview container');
      }

      const headings = Array.from(preview.querySelectorAll('h1, h2, h3, h4, h5, h6'));
      const heading = headings.find((element) => element.textContent?.includes(text)) as
        | HTMLElement
        | undefined;

      if (!heading) {
        return null;
      }

      const previewRect = preview.getBoundingClientRect();
      const headingRect = heading.getBoundingClientRect();
      return {
        top: headingRect.top - previewRect.top,
        bottom: headingRect.bottom - previewRect.top,
        height: preview.clientHeight,
      };
    },
    { text: headingText },
  );

  expect(bounds).not.toBeNull();
  expect(bounds!.top).toBeGreaterThanOrEqual(0);
  expect(bounds!.bottom).toBeLessThanOrEqual(bounds!.height);
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

    const editPage = new EditPage(page);
    await editPage.openFrontmatterPanel();

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

    const editPage = new EditPage(page);
    await editPage.openFrontmatterPanel();
    await addTag(page, 'release');
    await addProperty(page, 'status', 'draft');

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

    const editPage = new EditPage(page);
    await editPage.openFrontmatterPanel();
    await removeTag(page, 'leafwiki');
    await removeTag(page, 'andere');

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

    const editPage = new EditPage(page);
    await editPage.openFrontmatterPanel();

    // Remove the 'initial' tag
    await page
      .locator('.page-frontmatter-panel__chip')
      .filter({ hasText: 'initial' })
      .locator('button[aria-label*="Remove tag"]')
      .click();

    await addTag(page, 'updated');

    // Update the property value
    await page.locator('[data-testid="page-frontmatter-field-value-0"]').fill('two');

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

  test('editor-saves-properties-without-tags-and-resets-dirty-state', async ({ page }) => {
    const stamp = Date.now();
    const slug = `editor-properties-only-${stamp}`;

    await createPageWithMetadata(page, {
      title: `Editor Properties Only ${stamp}`,
      slug,
      content: 'Properties only content.',
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.clickEditPageButton();

    const saveButton = page.locator('button[data-testid="save-page-button"]');
    await saveButton.waitFor({ state: 'visible' });

    const editPage = new EditPage(page);
    await editPage.openFrontmatterPanel();
    await addProperty(page, 'status', 'draft');
    await expect(saveButton).toBeEnabled();

    await editPage.savePage();
    await expect(saveButton).toBeDisabled();
    await editPage.closeEditor();

    const propsToggle = page.locator('.page-metadata__props-toggle');
    await propsToggle.waitFor({ state: 'visible' });

    await expect(page.locator('.page-metadata')).toHaveClass(/page-metadata--two-col/);
    await expect(page.locator('.page-metadata__tag-chip')).toHaveCount(0);

    await propsToggle.click();
    await expect(
      page.locator('.page-metadata__prop-key').filter({ hasText: 'status' }),
    ).toBeVisible();
    await expect(
      page.locator('.page-metadata__prop-value').filter({ hasText: 'draft' }),
    ).toBeVisible();
  });

  test('editor-removes-all-properties-saves-and-resets-dirty-state', async ({ page }) => {
    const stamp = Date.now();
    const slug = `editor-remove-properties-${stamp}`;

    await createPageWithMetadata(page, {
      title: `Editor Remove Properties ${stamp}`,
      slug,
      content: 'Remove properties content.',
      properties: { status: 'draft', owner: 'alice' },
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.clickEditPageButton();

    const saveButton = page.locator('button[data-testid="save-page-button"]');
    await saveButton.waitFor({ state: 'visible' });

    const editPage = new EditPage(page);
    await editPage.openFrontmatterPanel();
    await expect(page.locator('[data-testid^="page-frontmatter-field-key-"]')).toHaveCount(2);
    await removeProperty(page, 1);
    await removeProperty(page, 0);
    await expect(saveButton).toBeEnabled();

    await editPage.savePage();
    await expect(saveButton).toBeDisabled();
    await editPage.closeEditor();

    await expect(page.locator('.page-metadata')).toHaveCount(0);

    const fetchedProperties = await page.evaluate(
      async ({ path, csrfScript }) => {
        const csrfToken = new Function(csrfScript)() as string;
        const response = await fetch(`/api/pages/by-path?path=${encodeURIComponent(path)}`, {
          credentials: 'include',
          headers: { 'X-CSRF-Token': csrfToken },
        });
        if (!response.ok) {
          throw new Error(`load failed: ${response.status}`);
        }
        const data = (await response.json()) as {
          properties?: Record<string, string>;
        };
        return data.properties ?? {};
      },
      { path: slug, csrfScript: getCsrfScript() },
    );

    expect(fetchedProperties).toEqual({});
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

    const editPage = new EditPage(page);
    await editPage.openFrontmatterPanel();
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

    const editPage = new EditPage(page);
    await editPage.openFrontmatterPanel();
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

    const editPage = new EditPage(page);
    await editPage.openFrontmatterPanel();

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

  test('editor-clicking-several-headings-does-not-jump-preview-to-top', async ({ page }) => {
    const stamp = Date.now();
    const slug = `editor-preview-image-heading-${stamp}`;
    const editorPreviewScrollFixture = readFileSync(editorPreviewScrollFixturePath, 'utf8');
    const headingChecks = [
      {
        editorLine: '## First Regular Heading',
        previewHeading: 'First Regular Heading',
      },
      {
        editorLine: '## Second Regular Heading',
        previewHeading: 'Second Regular Heading',
      },
      {
        editorLine: '## Third Regular Heading',
        previewHeading: 'Third Regular Heading',
      },
      {
        editorLine: '## Fourth Regular Heading',
        previewHeading: 'Fourth Regular Heading',
      },
      {
        editorLine: '## Inline Marker Heading One <span',
        previewHeading: 'Inline Marker Heading One',
      },
      {
        editorLine: '## Inline Marker Heading Two <span',
        previewHeading: 'Inline Marker Heading Two',
      },
      {
        editorLine: '## Inline Marker Heading Three <span',
        previewHeading: 'Inline Marker Heading Three',
      },
      {
        editorLine: '## Inline Marker Heading Four <span',
        previewHeading: 'Inline Marker Heading Four',
      },
      {
        editorLine: '## Inline Marker Heading Five <span',
        previewHeading: 'Inline Marker Heading Five',
      },
      {
        editorLine: '## Inline Marker Heading Six <span',
        previewHeading: 'Inline Marker Heading Six',
      },
      {
        editorLine: '## Final Regular Heading',
        previewHeading: 'Final Regular Heading',
      },
      {
        editorLine: '## Last Inline Marker Heading <span',
        previewHeading: 'Last Inline Marker Heading',
      },
    ];

    await createPageWithMetadata(page, {
      title: `Editor Preview Image Heading ${stamp}`,
      slug,
      content: editorPreviewScrollFixture,
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.clickEditPageButton();

    await expect(page.locator('#markdown-preview-container')).toContainText(
      'Last Inline Marker Heading',
    );

    const observedScrollTops: Array<{ heading: string; scrollTop: number }> = [];
    let furthestScrollTop = 0;

    for (let index = 0; index < headingChecks.length; index += 1) {
      const headingCheck = headingChecks[index];
      await clickEditorLineByText(page, headingCheck.editorLine);

      await expectPreviewHeadingVisible(page, headingCheck.previewHeading);

      const scrollTop = await getPreviewScrollTop(page);
      furthestScrollTop = Math.max(furthestScrollTop, scrollTop);
      observedScrollTops.push({
        heading: headingCheck.previewHeading,
        scrollTop,
      });

      if (furthestScrollTop > 300) {
        expect(
          scrollTop,
          `Expected preview not to jump back to top after clicking "${headingCheck.previewHeading}". Observed: ${JSON.stringify(
            observedScrollTops,
          )}`,
        ).toBeGreaterThan(100);
      }
    }
  });

  // ── Editor lifecycle ────────────────────────────────────────────────────────

  test('rename-via-tree-is-not-blocked-after-closing-editor', async ({ page }) => {
    const stamp = Date.now();
    const slug = `editor-rename-after-close-${stamp}`;
    const title = `Editor Rename After Close ${stamp}`;
    const renamedTitle = `${title} Renamed`;

    await createPageWithMetadata(page, {
      title,
      slug,
      content: 'Content.',
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.expectEditorStillOpen();
    await editPage.closeEditor();

    // Regression: the editor store used to keep `page` populated after the
    // editor unmounted, so renaming this same page via the tree's "..." menu
    // was permanently blocked by a stale "currently being edited" toast even
    // though the editor was already closed.
    const treeView = new TreeView(page);
    await treeView.openRenameDialogForPage(title);

    const metadataDialog = new EditPageMetadataDialog(page);
    await metadataDialog.fillTitle(renamedTitle);
    await metadataDialog.submit();

    await expect(page.getByText('currently being edited')).not.toBeVisible();
    await page.getByText('renamed successfully').waitFor({ state: 'visible' });
    await page.waitForURL(new RegExp(`/${slug}-renamed$`));
    await expect(page.locator('.breadcrumbs-nav__current')).toHaveText(renamedTitle);
  });
});

// ─── Line wrap ───────────────────────────────────────────────────────────────

test.describe('Editor line wrap', () => {
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

  test('editor-line-wrap-toggle-enables-and-disables-wrapping', async ({ page }) => {
    const stamp = Date.now();
    const slug = `editor-line-wrap-${stamp}`;

    await createPageWithMetadata(page, {
      title: `Editor Line Wrap ${stamp}`,
      slug,
      content: '',
    });

    // Reset stored editor settings so the test always starts with the default (lineWrap: true)
    await page.evaluate(() => localStorage.removeItem('leafwiki-editor-settings'));

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.clickEditPageButton();

    const cmContent = page.locator('.cm-content');
    await cmContent.waitFor({ state: 'visible' });

    // Default: line wrap is enabled
    await expect(cmContent).toHaveClass(/cm-lineWrapping/);

    // Disable line wrap
    await page.locator('[data-testid="toggle-line-wrap-button"]').click();
    await expect(cmContent).not.toHaveClass(/cm-lineWrapping/);

    // Re-enable line wrap
    await page.locator('[data-testid="toggle-line-wrap-button"]').click();
    await expect(cmContent).toHaveClass(/cm-lineWrapping/);
  });
});

// ─── Formatting ───────────────────────────────────────────────────────────────

test.describe('Editor formatting', () => {
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

  test('editor-bold-via-keyboard', async ({ page }) => {
    const stamp = Date.now();
    const slug = `editor-bold-keyboard-${stamp}`;

    await createPageWithMetadata(page, {
      title: `Editor Bold Keyboard ${stamp}`,
      slug,
      content: '',
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await page.locator('.cm-editor').click();
    await page.keyboard.press('Control+b');
    await page.keyboard.type('Bold Text');
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('ArrowRight');

    await editPage.savePage();
    await editPage.closeEditor();

    await page.locator('article strong').getByText('Bold Text').waitFor({ state: 'visible' });
  });

  test('editor-italic-via-keyboard', async ({ page }) => {
    const stamp = Date.now();
    const slug = `editor-italic-keyboard-${stamp}`;

    await createPageWithMetadata(page, {
      title: `Editor Italic Keyboard ${stamp}`,
      slug,
      content: '',
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await page.locator('.cm-editor').click();
    await page.keyboard.press('Control+i');
    await page.keyboard.type('Italic Text');
    await page.keyboard.press('ArrowRight');

    await editPage.savePage();
    await editPage.closeEditor();

    await page.locator('article em').getByText('Italic Text').waitFor({ state: 'visible' });
  });

  test('editor-bold-via-toolbar-button', async ({ page }) => {
    const stamp = Date.now();
    const slug = `editor-bold-button-${stamp}`;

    await createPageWithMetadata(page, {
      title: `Editor Bold Button ${stamp}`,
      slug,
      content: '',
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await page.locator('.cm-editor').click();
    await page.locator('[data-testid="format-bold-button"]').click();
    await page.keyboard.type('Bold Text');
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('ArrowRight');

    await editPage.savePage();
    await editPage.closeEditor();

    await page.locator('article strong').getByText('Bold Text').waitFor({ state: 'visible' });
  });

  test('editor-italic-via-toolbar-button', async ({ page }) => {
    const stamp = Date.now();
    const slug = `editor-italic-button-${stamp}`;

    await createPageWithMetadata(page, {
      title: `Editor Italic Button ${stamp}`,
      slug,
      content: '',
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await page.locator('.cm-editor').click();
    await page.locator('[data-testid="format-italic-button"]').click();
    await page.keyboard.type('Italic Text');
    await page.keyboard.press('ArrowRight');

    await editPage.savePage();
    await editPage.closeEditor();

    await page.locator('article em').getByText('Italic Text').waitFor({ state: 'visible' });
  });
});
