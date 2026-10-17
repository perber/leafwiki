import test, { Page, expect } from '@playwright/test';
import DeletePageDialog from '../pages/DeletePageDialog';
import EditPage from '../pages/EditPage';
import EditPageMetadataDialog from '../pages/EditPageMetadataDialog';
import LoginPage from '../pages/LoginPage';
import MovePageDialog from '../pages/MovePageDialog';
import ViewPage from '../pages/ViewPage';
import { browserFetch, expectOk, pageByPathUrl } from '../helpers/api';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

// ─── API helpers ──────────────────────────────────────────────────────────────

async function createPage(
  page: Page,
  input: { title: string; slug: string; content?: string; parentSlug?: string },
): Promise<{ id: string; version: string }> {
  const { title, slug, content, parentSlug } = input;
  let parentId: string | null = null;
  if (parentSlug) {
    parentId = expectOk(
      await browserFetch<{ id: string }>(page, pageByPathUrl(parentSlug)),
      'parent lookup',
    ).id;
  }

  const created = expectOk(
    await browserFetch<{ id: string; version: string }>(page, '/api/pages', {
      method: 'POST',
      body: { parentId, title, slug, kind: 'page' },
    }),
    'create',
  );

  if (content !== undefined) {
    return expectOk(
      await browserFetch<{ id: string; version: string }>(page, `/api/pages/${created.id}`, {
        method: 'PUT',
        body: { version: created.version, title, slug, content, tags: [], properties: {} },
      }),
      'update',
    );
  }
  return created;
}

async function refactorPreview(
  page: Page,
  pageSlug: string,
  body: Record<string, unknown>,
): Promise<{
  counts: { affectedPages: number; matchedLinks: number };
  affectedPages: Array<{ fromTitle: string; matchedPaths: string[] }>;
}> {
  const { id } = expectOk(
    await browserFetch<{ id: string }>(page, pageByPathUrl(pageSlug)),
    'lookup',
  );
  return expectOk(
    await browserFetch<{
      counts: { affectedPages: number; matchedLinks: number };
      affectedPages: Array<{ fromTitle: string; matchedPaths: string[] }>;
    }>(page, `/api/pages/${id}/refactor/preview`, { method: 'POST', body }),
    'preview',
  );
}

async function applyRefactorRename(
  page: Page,
  pageSlug: string,
  newTitle: string,
  newSlug: string,
  rewriteLinks: boolean,
): Promise<void> {
  const { id, version } = expectOk(
    await browserFetch<{ id: string; version: string }>(page, pageByPathUrl(pageSlug)),
    'lookup',
  );
  expectOk(
    await browserFetch(page, `/api/pages/${id}/refactor/apply`, {
      method: 'POST',
      body: { kind: 'rename', version, title: newTitle, slug: newSlug, rewriteLinks },
    }),
    'apply',
  );
}

async function getPageContent(page: Page, slug: string): Promise<string> {
  return expectOk(await browserFetch<{ content: string }>(page, pageByPathUrl(slug)), 'lookup')
    .content;
}

/** Polls the refactor preview endpoint until affectedPages reaches the expected count. */
async function waitForAffectedPages(
  page: Page,
  pageSlug: string,
  previewBody: Record<string, unknown>,
  expectedCount: number,
) {
  await expect
    .poll(
      async () => {
        const preview = await refactorPreview(page, pageSlug, previewBody);
        return preview.counts.affectedPages;
      },
      { timeout: 15000 },
    )
    .toBe(expectedCount);
}

// ─── Suite ────────────────────────────────────────────────────────────────────

test.describe('WikiLink [[Title]] refactoring and link status', () => {
  test.beforeEach(async ({ page }) => {
    await new LoginPage(page).goto();
    await new LoginPage(page).login(user, password);
    await new ViewPage(page).expectUserLoggedIn();
  });

  test.afterEach(async ({ page }) => {
    await new ViewPage(page).logout();
  });

  // ── Rename: preview and apply via API ─────────────────────────────────────

  test('rename-preview-includes-wikilink-title-as-affected-page', async ({ page }) => {
    const s = Date.now();
    const targetTitle = `Rename Target ${s}`;
    const targetSlug = `rename-target-${s}`;
    const refTitle = `Rename Ref ${s}`;
    const refSlug = `rename-ref-${s}`;
    const newTitle = `Rename Target New ${s}`;
    const newSlug = `rename-target-new-${s}`;

    await createPage(page, { title: targetTitle, slug: targetSlug });
    await createPage(page, { title: refTitle, slug: refSlug, content: `[[${targetTitle}]]` });

    const previewBody = { kind: 'rename', title: newTitle, slug: newSlug };
    await waitForAffectedPages(page, targetSlug, previewBody, 1);

    const preview = await refactorPreview(page, targetSlug, previewBody);
    expect(preview.counts.affectedPages).toBe(1);
    expect(preview.affectedPages[0].fromTitle).toBe(refTitle);
    // The matched path entry must use wiki-link syntax, not a raw route path.
    expect(preview.affectedPages[0].matchedPaths).toContain(`[[${targetTitle}]]`);
  });

  test('rename-rewrite-updates-wikilink-title-in-referencing-page', async ({ page }) => {
    const s = Date.now();
    const targetTitle = `Rewrite Target ${s}`;
    const targetSlug = `rewrite-target-${s}`;
    const newTitle = `Rewrite Target Renamed ${s}`;
    const newSlug = `rewrite-target-renamed-${s}`;
    const refSlug = `rewrite-ref-${s}`;

    await createPage(page, { title: targetTitle, slug: targetSlug });
    await createPage(page, {
      title: `Rewrite Ref ${s}`,
      slug: refSlug,
      content: `[[${targetTitle}]]`,
    });

    await waitForAffectedPages(
      page,
      targetSlug,
      { kind: 'rename', title: newTitle, slug: newSlug },
      1,
    );

    await applyRefactorRename(page, targetSlug, newTitle, newSlug, true);

    const content = await getPageContent(page, refSlug);
    expect(content).toContain(`[[${newTitle}]]`);
    expect(content).not.toContain(`[[${targetTitle}]]`);
  });

  // ── Rename: refactor dialog via UI ────────────────────────────────────────

  test('rename-refactor-dialog-shows-wikilink-page-with-correct-matched-path', async ({ page }) => {
    const s = Date.now();
    const targetTitle = `UI Rename Target ${s}`;
    const targetSlug = `ui-rename-target-${s}`;
    const newTitle = `UI Rename Target New ${s}`;
    const newSlug = `ui-rename-target-new-${s}`;
    const refTitle = `UI Rename Ref ${s}`;
    const refSlug = `ui-rename-ref-${s}`;

    await createPage(page, { title: targetTitle, slug: targetSlug });
    await createPage(page, { title: refTitle, slug: refSlug, content: `[[${targetTitle}]]` });

    // Wait for the link index before navigating so the dialog shows correct data.
    await waitForAffectedPages(
      page,
      targetSlug,
      { kind: 'rename', title: newTitle, slug: newSlug },
      1,
    );

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${targetSlug}`);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.openMetadataDialog();

    const metaDialog = new EditPageMetadataDialog(page);
    await metaDialog.fillTitle(newTitle);
    await metaDialog.fillSlug(newSlug);
    await metaDialog.submit();

    // Auto-save skips slug changes — trigger a manual save.
    // Wait until the toolbar reflects the metadata change so the manual save
    // click actually starts the refactor flow.
    const saveButton = page.locator('button[data-testid="save-page-button"]');
    await expect(saveButton).toBeEnabled();
    // Click the button without awaiting completion: the refactor dialog appears
    // mid-save and must be confirmed before the save promise resolves.
    await saveButton.click();

    // Slug changed → refactor preview dialog must appear.
    const movePageDialog = new MovePageDialog(page);
    await movePageDialog.expectRefactorDialogVisible();
    await movePageDialog.expectAffectedPagesCount(1);
    await movePageDialog.expectAffectedPageTitle(refTitle);

    // The matched-path column must show the title, not raw [[Title]] wiki-link syntax.
    await expect(
      page.locator('[data-testid="page-refactor-dialog-affected-page-matches"]'),
    ).toContainText(targetTitle);

    await movePageDialog.confirmRefactorDialog();

    // Wait for the save to complete after the dialog is confirmed.
    await page.getByText('Page saved successfully').last().waitFor({ state: 'visible' });

    await editPage.closeEditor();

    // After rewrite, the stored ref page content must use the new title.
    const refContent = await getPageContent(page, refSlug);
    expect(refContent).toContain(`[[${newTitle}]]`);
    expect(refContent).not.toContain(`[[${targetTitle}]]`);
  });

  // ── Move: path-hint wikilink appears in refactor dialog ───────────────────

  test('move-refactor-dialog-shows-path-hint-wikilink-as-affected-page', async ({ page }) => {
    const s = Date.now();
    const parentSlug = `move-parent-${s}`;
    const targetSlug = `move-target-${s}`;
    const refTitle = `Move Ref ${s}`;
    const refSlug = `move-ref-${s}`;
    const fullPath = `${parentSlug}/${targetSlug}`;

    await createPage(page, { title: `Move Parent ${s}`, slug: parentSlug });
    await createPage(page, { title: `Move Target ${s}`, slug: targetSlug, parentSlug });
    // Path-hint wikilink: [[parent/child]] resolves via route-path lookup.
    await createPage(page, { title: refTitle, slug: refSlug, content: `[[${fullPath}]]` });

    await waitForAffectedPages(page, fullPath, { kind: 'move', parentId: null }, 1);

    const preview = await refactorPreview(page, fullPath, { kind: 'move', parentId: null });
    expect(preview.counts.affectedPages).toBe(1);
    expect(preview.affectedPages[0].fromTitle).toBe(refTitle);
    // Path-hint wikilinks should appear with their [[path/hint]] syntax in the preview.
    const allMatches = preview.affectedPages[0].matchedPaths;
    expect(allMatches.some((p) => p.includes(targetSlug))).toBe(true);
  });

  // ── Ambiguous links: not broken ───────────────────────────────────────────

  test('ambiguous-wikilink-not-shown-as-broken-in-link-panel', async ({ page }) => {
    const s = Date.now();
    const sharedTitle = `Ambiguous Broken ${s}`;
    const slug1 = `ambiguous-broken-1-${s}`;
    const slug2 = `ambiguous-broken-2-${s}`;
    const sourceSlug = `ambiguous-broken-source-${s}`;

    await createPage(page, { title: sharedTitle, slug: slug1 });
    await createPage(page, { title: sharedTitle, slug: slug2 });
    await createPage(page, {
      title: `Ambiguous Source ${s}`,
      slug: sourceSlug,
      content: `[[${sharedTitle}]]`,
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${sourceSlug}`);

    // Wait for backlinks panel to finish loading (no more "Loading…" text).
    await expect
      .poll(async () => page.locator('.backlinks__content').textContent(), { timeout: 15000 })
      .not.toContain('Loading');

    // "Broken links" count must be 0 — ambiguous [[Title]] is not a broken link.
    const brokenGroupTitle = page
      .locator('.backlinks__group-title')
      .filter({ hasText: 'Broken links' });
    await expect(brokenGroupTitle).toContainText('0');

    // No broken-link items should be visible.
    await expect(page.locator('.backlinks__item--broken')).toHaveCount(0);
  });

  // ── Ambiguous links: appear as backlinks for all matching pages ───────────

  test('ambiguous-wikilink-appears-as-backlink-for-both-matching-pages', async ({ page }) => {
    const s = Date.now();
    const sharedTitle = `Ambiguous Backlink ${s}`;
    const slug1 = `ambiguous-bl-1-${s}`;
    const slug2 = `ambiguous-bl-2-${s}`;
    const sourceTitle = `Ambiguous BL Source ${s}`;
    const sourceSlug = `ambiguous-bl-source-${s}`;

    await createPage(page, { title: sharedTitle, slug: slug1 });
    await createPage(page, { title: sharedTitle, slug: slug2 });
    await createPage(page, { title: sourceTitle, slug: sourceSlug, content: `[[${sharedTitle}]]` });

    const viewPage = new ViewPage(page);

    for (const slug of [slug1, slug2]) {
      await viewPage.goto(`/${slug}`);

      // Wait for the "Referenced by" badge to show at least 1 entry.
      await expect
        .poll(
          async () => {
            const el = page.locator('.backlinks__group-title').filter({ hasText: 'Referenced by' });
            return el.textContent();
          },
          { timeout: 15000 },
        )
        .toMatch(/Referenced by\D*[1-9]/);

      // The source page must appear as a backlink.
      await expect(page.locator('.backlinks__item').filter({ hasText: sourceTitle })).toBeVisible();
    }
  });

  // ── Delete dialog: [[Title]] backlinks show warning ───────────────────────

  test('delete-dialog-shows-wikilink-backlink-warning', async ({ page }) => {
    const s = Date.now();
    const targetTitle = `Del Wikilink Target ${s}`;
    const targetSlug = `del-wikilink-target-${s}`;
    const refTitle = `Del Wikilink Ref ${s}`;
    const refSlug = `del-wikilink-ref-${s}`;

    await createPage(page, { title: targetTitle, slug: targetSlug });
    await createPage(page, { title: refTitle, slug: refSlug, content: `[[${targetTitle}]]` });

    // Wait until the link index has registered the [[Title]] backlink
    // (reuse the rename preview as a proxy for index readiness).
    await waitForAffectedPages(
      page,
      targetSlug,
      { kind: 'rename', title: `${targetTitle} X`, slug: `${targetSlug}-x` },
      1,
    );

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${targetSlug}`);
    await viewPage.clickDeletePageButton();

    const deleteDialog = new DeletePageDialog(page);
    await deleteDialog.waitForVisible();
    await deleteDialog.expectBacklinksWarningVisible();
    await deleteDialog.expectBacklinkTitle(refTitle);
    await deleteDialog.abortDeletion();
  });
});
