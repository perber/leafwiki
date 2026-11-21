import { type Page, expect, test } from '@playwright/test';
import { createPageViaApi, loggedIn } from '../helpers/apiContext';
import EditPage from '../pages/EditPage';
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

// Collects the errors a stale DOM node or range would raise.
function collectRenderErrors(page: Page) {
  const errors: string[] = [];
  page.on('console', (message) => {
    if (message.type() !== 'error') return;
    const text = message.text();
    if (
      /insertBefore|removeChild|NotFoundError|IndexSizeError|Markdown preview failed to render/i.test(
        text,
      )
    ) {
      errors.push(text);
    }
  });
  page.on('pageerror', (error) => errors.push(error.message));
  return errors;
}

async function login(page: Page) {
  await new LoginPage(page).loginViaApi(user, password);
  await new ViewPage(page).expectUserLoggedIn();
}

function navigateInApp(page: Page, path: string) {
  return page.evaluate((target) => {
    window.history.pushState({}, '', target);
    window.dispatchEvent(new PopStateEvent('popstate'));
  }, path);
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

  const errors = collectRenderErrors(page);

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

test('rapid in-app navigation between ?q= pages highlights only the last match', async ({
  page,
}) => {
  const stamp = Date.now();
  const api = await loggedIn();
  const slugs: string[] = [];
  for (const n of [1, 2, 3]) {
    const slug = `search-rapid-${n}-${stamp}`;
    slugs.push(slug);
    await createPageViaApi(api, {
      title: `Rapid ${n}`,
      slug,
      content: `# Rapid ${n}\n\nSome **bold** text and match${n}x${stamp} *here*.\n`,
    });
  }
  await api.dispose();

  await login(page);
  const errors = collectRenderErrors(page);
  await page.goto(toAppPath(`/${slugs[0]}?q=match1x${stamp}`));
  await expect.poll(() => searchHighlightTexts(page)).toEqual([`match1x${stamp}`]);

  // No waiting in between: each effect is cleaned up before its layout settles.
  for (const n of [2, 3, 1, 3]) {
    await navigateInApp(page, toAppPath(`/${slugs[n - 1]}?q=match${n}x${stamp}`));
  }
  await expect(page.locator('article>h1')).toHaveText('Rapid 3');
  await expect.poll(() => searchHighlightTexts(page)).toEqual([`match3x${stamp}`]);
  await page.waitForTimeout(1500);
  expect(await searchHighlightTexts(page)).toEqual([`match3x${stamp}`]);
  expect(errors).toEqual([]);
});

test('a heading hash wins over ?q= and nothing is highlighted', async ({ page }) => {
  const stamp = Date.now();
  const needle = `hashneedle${stamp}`;
  const slug = `search-hash-${stamp}`;
  const filler = Array.from({ length: 60 }, (_, i) => `Filler paragraph ${i}.`).join('\n\n');
  const api = await loggedIn();
  await createPageViaApi(api, {
    title: 'Hash Page',
    slug,
    content: `# Hash Page\n\n${needle} at the top.\n\n${filler}\n\n## Target Section\n\nEnd.\n`,
  });
  await api.dispose();

  await login(page);
  await page.goto(toAppPath(`/${slug}?q=${needle}#target-section`));
  await expect(page.getByRole('heading', { name: 'Target Section' })).toBeInViewport();
  await page.waitForTimeout(1500);
  expect(await searchHighlightTexts(page)).toEqual([]);
  await expect(page.getByRole('heading', { name: 'Target Section' })).toBeInViewport();
});

test('editing the page under an active ?q= highlight re-renders without errors', async ({
  page,
}) => {
  const stamp = Date.now();
  const needle = `editneedle${stamp}`;
  const slug = `search-edit-${stamp}`;
  const api = await loggedIn();
  await createPageViaApi(api, {
    title: 'Edit Page',
    slug,
    content: `# Edit Page\n\nBefore ${needle} after **bold**.\n`,
  });
  await api.dispose();

  await login(page);
  const errors = collectRenderErrors(page);
  await page.goto(toAppPath(`/${slug}?q=${needle}`));
  await expect.poll(() => searchHighlightTexts(page)).toEqual([needle]);

  await new ViewPage(page).clickEditPageButton();
  const editPage = new EditPage(page);
  await page.locator('.cm-editor').click();
  await page.keyboard.press('ControlOrMeta+Home');
  await editPage.writeContent('Prefix line.\n\n');
  await editPage.savePage();
  await editPage.closeEditor();

  await expect(page.locator('article')).toContainText('Prefix line.');
  await expect(page.locator('article')).toContainText(`Before ${needle} after bold.`);
  // Closing the editor returns to the page without ?q=, so the old highlight is gone.
  await expect(page).not.toHaveURL(/[?&]q=/);
  await expect.poll(() => searchHighlightTexts(page)).toEqual([]);

  // A new query on the re-rendered content highlights the new text node.
  await navigateInApp(page, toAppPath(`/${slug}?q=prefix line`));
  await expect.poll(() => searchHighlightTexts(page)).toEqual(['Prefix line']);
  expect(errors).toEqual([]);
});

test('a ?q= match inside a ==marker== keeps the highlights panel working', async ({ page }) => {
  const stamp = Date.now();
  const needle = `marked${stamp}`;
  const slug = `search-marker-${stamp}`;
  const other = `search-marker-other-${stamp}`;
  const api = await loggedIn();
  await createPageViaApi(api, {
    title: 'Marker Page',
    slug,
    content: `# Marker Page\n\nSee ==the ${needle} text== and ==second==. [next](/${other})\n`,
  });
  await createPageViaApi(api, {
    title: 'Marker Other',
    slug: other,
    content: '# Marker Other\n\nOnly ==other mark==.\n',
  });
  await api.dispose();

  await login(page);
  const errors = collectRenderErrors(page);
  await page.goto(toAppPath(`/${slug}?q=${needle}`));
  await expect.poll(() => searchHighlightTexts(page)).toEqual([needle]);
  const labels = page.locator('[data-testid^="toc-highlight-"]');
  await expect(labels).toHaveText([`the ${needle} text`, 'second']);

  await page.locator('article a', { hasText: 'next' }).click();
  await expect(labels).toHaveText(['other mark']);
  await page.goBack();
  await expect(labels).toHaveText([`the ${needle} text`, 'second']);
  expect(errors).toEqual([]);
});
