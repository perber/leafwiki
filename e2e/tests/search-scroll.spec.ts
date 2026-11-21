import { type Page, expect, test } from '@playwright/test';
import { createPageViaApi, loggedIn } from '../helpers/apiContext';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';
import { toAppPath } from '../pages/appPath';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

// The match is painted with the CSS Custom Highlight API, not a DOM element.
function searchHighlightTexts(page: Page) {
  return page.evaluate(() => {
    const highlight = CSS.highlights?.get('leafwiki-search-query');
    return highlight ? [...highlight].map((range) => range.toString()) : [];
  });
}

// #712: opening a page from a search result (?q=) scrolls to the first match.
test('opening a page with ?q= scrolls to and highlights the first match', async ({ page }) => {
  const stamp = Date.now();
  const needle = `needle${stamp}`;
  const slug = `search-scroll-${stamp}`;
  const filler = Array.from({ length: 80 }, (_, i) => `Filler paragraph ${i}.`).join('\n\n');

  const api = await loggedIn();
  await createPageViaApi(api, {
    title: 'Long Page',
    slug,
    content: `# Long Page\n\n${filler}\n\nThe ${needle} is down here.\n`,
  });
  await api.dispose();

  const loginPage = new LoginPage(page);
  await loginPage.loginViaApi(user, password);
  await new ViewPage(page).expectUserLoggedIn();

  await page.goto(toAppPath(`/${slug}?q=${needle}`));
  await expect(page.locator('article>h1')).toHaveText('Long Page');

  await expect.poll(() => searchHighlightTexts(page)).toEqual([needle]);
  await expect(page.locator('article p', { hasText: needle })).toBeInViewport();
  await expect
    .poll(() => page.evaluate(() => document.getElementById('scroll-container')?.scrollTop ?? 0))
    .toBeGreaterThan(0);
});

// The search-query highlight must not change the DOM React rendered: switching
// pages after it was applied used to throw "insertBefore ... not a child of
// this node" and trip the preview error boundary.
test('switching pages after a ?q= highlight keeps the preview rendering', async ({ page }) => {
  const stamp = Date.now();
  const needle = `needle${stamp}`;
  const fromSlug = `search-switch-from-${stamp}`;
  const toSlug = `search-switch-to-${stamp}`;

  const api = await loggedIn();
  // Mixed inline content, so React keeps separate text nodes next to elements.
  await createPageViaApi(api, {
    title: 'Switch From',
    slug: fromSlug,
    content: `# Switch From\n\nThe ${needle} sits **between** two *inline* nodes.\n\n[Go to target](/${toSlug})\n`,
  });
  await createPageViaApi(api, {
    title: 'Switch To',
    slug: toSlug,
    content:
      '# Switch To\n\n*Leading* text with **bold** and the target body here.\n\nSecond paragraph of the target.\n',
  });
  await api.dispose();

  const loginPage = new LoginPage(page);
  await loginPage.loginViaApi(user, password);
  await new ViewPage(page).expectUserLoggedIn();

  const errors: string[] = [];
  page.on('console', (message) => {
    if (message.type() !== 'error') return;
    const text = message.text();
    if (/insertBefore|removeChild|NotFoundError|Markdown preview failed to render/i.test(text)) {
      errors.push(text);
    }
  });
  page.on('pageerror', (error) => errors.push(error.message));

  await page.goto(toAppPath(`/${fromSlug}?q=${needle}`));
  await expect(page.locator('article>h1')).toHaveText('Switch From');
  await expect.poll(() => searchHighlightTexts(page)).toEqual([needle]);

  await page.locator('article').getByRole('link', { name: 'Go to target' }).click();
  await expect(page.locator('article>h1')).toHaveText('Switch To');
  await expect(page.locator('article')).toContainText('the target body here');
  await expect(page.locator('article')).toContainText('Second paragraph of the target.');

  await page.goBack();
  await expect(page.locator('article>h1')).toHaveText('Switch From');
  await expect(page.locator('article')).toContainText(
    `The ${needle} sits between two inline nodes.`,
  );

  await page.goForward();
  await expect(page.locator('article>h1')).toHaveText('Switch To');
  await expect(page.locator('article')).toContainText('Second paragraph of the target.');

  expect(errors).toEqual([]);
});
