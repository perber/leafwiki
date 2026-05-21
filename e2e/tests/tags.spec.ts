import test, { expect } from '@playwright/test';
import LoginPage from '../pages/LoginPage';
import TagsView from '../pages/TagsView';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

async function createPageWithTags(
  page: import('@playwright/test').Page,
  input: { title: string; slug: string; content: string; tags: string[] },
) {
  await page.evaluate(async ({ title, slug, content, tags }) => {
    function getCsrf(): string | null {
      const m =
        document.cookie.match(/(?:^|;\s*)__Host-leafwiki_csrf=([^;]+)/) ??
        document.cookie.match(/(?:^|;\s*)leafwiki_csrf=([^;]+)/);
      if (!m) return null;
      try {
        return decodeURIComponent(m[1]);
      } catch {
        return m[1];
      }
    }
    const csrf = getCsrf();
    if (!csrf) throw new Error('Missing CSRF token');

    const cr = await fetch('/api/pages', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
      body: JSON.stringify({ parentId: null, title, slug, kind: 'page' }),
    });
    if (!cr.ok) throw new Error(`create failed: ${cr.status}`);
    const created = (await cr.json()) as { id: string; version: string };

    const ur = await fetch(`/api/pages/${created.id}`, {
      method: 'PUT',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
      body: JSON.stringify({
        version: created.version,
        title,
        slug,
        content,
        tags,
        properties: {},
      }),
    });
    if (!ur.ok) throw new Error(`update failed: ${ur.status}`);
  }, input);
}

test.describe('tags panel', () => {
  test.beforeEach(async ({ page }) => {
    const loginPage = new LoginPage(page);
    await loginPage.goto();
    await loginPage.login(user, password);
    const viewPage = new ViewPage(page);
    await viewPage.expectUserLoggedIn();
  });

  test('clicking a tag filter shows matching result pages', async ({ page }) => {
    const stamp = Date.now();
    const matchTag = `e2e-match-${stamp}`;
    const otherTag = `e2e-other-${stamp}`;

    await createPageWithTags(page, {
      title: `Match A ${stamp}`,
      slug: `match-a-${stamp}`,
      content: 'First matching page content.',
      tags: [matchTag],
    });
    await createPageWithTags(page, {
      title: `Match B ${stamp}`,
      slug: `match-b-${stamp}`,
      content: 'Second matching page content.',
      tags: [matchTag],
    });
    await createPageWithTags(page, {
      title: `Other ${stamp}`,
      slug: `other-${stamp}`,
      content: 'Page with a different tag.',
      tags: [otherTag],
    });

    const tagsView = new TagsView(page);
    await tagsView.open();
    await tagsView.clickTagFilter(matchTag);

    await tagsView.expectChipVisible(matchTag);
    await tagsView.waitForResults();
    await tagsView.expectResultVisible(`Match A ${stamp}`);
    await tagsView.expectResultVisible(`Match B ${stamp}`);
    await tagsView.expectResultNotVisible(`Other ${stamp}`);
  });

  test('AND logic: multiple tag filters narrow results', async ({ page }) => {
    const stamp = Date.now();
    const tagA = `e2e-and-a-${stamp}`;
    const tagB = `e2e-and-b-${stamp}`;

    await createPageWithTags(page, {
      title: `Both Tags ${stamp}`,
      slug: `both-${stamp}`,
      content: 'Has both tags.',
      tags: [tagA, tagB],
    });
    await createPageWithTags(page, {
      title: `Only A ${stamp}`,
      slug: `only-a-${stamp}`,
      content: 'Has only tag A.',
      tags: [tagA],
    });

    const tagsView = new TagsView(page);
    await tagsView.open();

    await tagsView.clickTagFilter(tagA);
    await tagsView.waitForResults();
    await tagsView.expectResultVisible(`Both Tags ${stamp}`);
    await tagsView.expectResultVisible(`Only A ${stamp}`);

    await tagsView.clickTagFilter(tagB);
    await tagsView.waitForResults();
    await tagsView.expectResultVisible(`Both Tags ${stamp}`);
    await tagsView.expectResultNotVisible(`Only A ${stamp}`);
  });

  test('no results message shown for non-overlapping tag filters', async ({ page }) => {
    const stamp = Date.now();
    const tagA = `e2e-empty-a-${stamp}`;
    const tagB = `e2e-empty-b-${stamp}`;

    await createPageWithTags(page, {
      title: `Only A ${stamp}`,
      slug: `only-a-empty-${stamp}`,
      content: 'Page body.',
      tags: [tagA],
    });
    await createPageWithTags(page, {
      title: `Only B ${stamp}`,
      slug: `only-b-empty-${stamp}`,
      content: 'Another page body.',
      tags: [tagB],
    });

    const tagsView = new TagsView(page);
    await tagsView.open();
    const currentUrl = new URL(page.url());
    currentUrl.searchParams.delete('tags');
    currentUrl.searchParams.append('tags', tagA);
    currentUrl.searchParams.append('tags', tagB);
    await page.goto(currentUrl.toString());
    await tagsView.open();
    await tagsView.waitForEmptyState();
    await expect(tagsView.getResultsList()).not.toBeVisible();
  });

  test('clear filter removes results', async ({ page }) => {
    const stamp = Date.now();
    const tag = `e2e-clear-${stamp}`;

    await createPageWithTags(page, {
      title: `Clear Test ${stamp}`,
      slug: `clear-test-${stamp}`,
      content: 'Will be cleared.',
      tags: [tag],
    });

    const tagsView = new TagsView(page);
    await tagsView.open();
    await tagsView.clickTagFilter(tag);
    await tagsView.waitForResults();
    await tagsView.expectResultVisible(`Clear Test ${stamp}`);

    await tagsView.clearFilter();

    await expect(tagsView.getResultsList()).not.toBeVisible();
    await tagsView.expectChipNotVisible(tag);
  });

  test('removing the last selected tag clears results', async ({ page }) => {
    const stamp = Date.now();
    const tag = `e2e-remove-last-${stamp}`;

    await createPageWithTags(page, {
      title: `Remove Last ${stamp}`,
      slug: `remove-last-${stamp}`,
      content: 'Will disappear when the last tag is removed.',
      tags: [tag],
    });

    const tagsView = new TagsView(page);
    await tagsView.open();
    await tagsView.clickTagFilter(tag);
    await tagsView.waitForResults();
    await tagsView.expectResultVisible(`Remove Last ${stamp}`);

    await tagsView.clickTagFilter(tag);

    await expect(tagsView.getResultsList()).not.toBeVisible();
    await tagsView.expectChipNotVisible(tag);
  });

  test('opening a search result keeps active tags in the URL', async ({ page }) => {
    const stamp = Date.now();
    const primaryTag = `e2e-click-primary-${stamp}`;
    const secondaryTag = `e2e-click-secondary-${stamp}`;

    await createPageWithTags(page, {
      title: `Tag Click Test ${stamp}`,
      slug: `tag-click-${stamp}`,
      content: 'Page with two tags.',
      tags: [primaryTag, secondaryTag],
    });

    const tagsView = new TagsView(page);
    await tagsView.open();
    await tagsView.clickTagFilter(primaryTag);
    await tagsView.waitForResults();
    await tagsView.clickTagFilter(secondaryTag);
    await tagsView.waitForResults();

    const result = page
      .locator('a[data-testid^="search-result-card-"]')
      .filter({ hasText: `Tag Click Test ${stamp}` })
      .first();
    await result.click();

    await expect(page).toHaveURL(new RegExp(`tags=${primaryTag}`));
    await expect(page).toHaveURL(new RegExp(`tags=${secondaryTag}`));
  });

  test('result card shows excerpt', async ({ page }) => {
    const stamp = Date.now();
    const tag = `e2e-excerpt-${stamp}`;
    const excerptText = `Unique excerpt text for ${stamp}`;

    await createPageWithTags(page, {
      title: `Excerpt Page ${stamp}`,
      slug: `excerpt-page-${stamp}`,
      content: excerptText,
      tags: [tag],
    });

    const tagsView = new TagsView(page);
    await tagsView.open();
    await tagsView.clickTagFilter(tag);
    await tagsView.waitForResults();

    const excerpt = page.locator('.search-result-card__excerpt').filter({ hasText: excerptText });
    await expect(excerpt).toBeVisible();
  });

  test('deselecting one of multiple tags broadens results again', async ({ page }) => {
    const stamp = Date.now();
    const tagA = `e2e-bs-a-${stamp}`;
    const tagB = `e2e-bs-b-${stamp}`;

    await createPageWithTags(page, {
      title: `Backspace Test ${stamp}`,
      slug: `backspace-test-${stamp}`,
      content: 'Backspace test page.',
      tags: [tagA, tagB],
    });

    const tagsView = new TagsView(page);
    await tagsView.open();

    await tagsView.clickTagFilter(tagA);
    await tagsView.waitForResults();
    await tagsView.expectChipVisible(tagA);
    await tagsView.expectResultVisible(`Backspace Test ${stamp}`);

    await tagsView.clickTagFilter(tagB);
    await tagsView.expectChipVisible(tagB);
    await tagsView.waitForResults();

    await tagsView.clickTagFilter(tagB);

    await tagsView.expectChipNotVisible(tagB);
    await tagsView.expectChipVisible(tagA);
    await tagsView.waitForResults();
    await tagsView.expectResultVisible(`Backspace Test ${stamp}`);
  });
});
