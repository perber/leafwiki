import { APIRequestContext, Page, expect, test } from '@playwright/test';
import { createPageViaApi, csrfHeader, loggedIn, setAlwaysShowToc } from '../helpers/apiContext';
import LoginPage from '../pages/LoginPage';
import EditPage from '../pages/EditPage';
import ViewPage from '../pages/ViewPage';
import { toAppPath } from '../pages/appPath';

// The right-hand page panel lists the page's headings, its highlights and its
// attachments as separate sections (ADR-0021). These cover how they combine
// with the instance-wide "always show TOC" setting.

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';
const fileName = 'notes.txt';

async function uploadAttachment(api: APIRequestContext, pageId: string) {
  const resp = await api.post(`/api/pages/${pageId}/assets`, {
    headers: await csrfHeader(api),
    multipart: {
      file: { name: fileName, mimeType: 'text/plain', buffer: Buffer.from('hello') },
    },
  });
  expect(resp.status(), 'upload attachment').toBe(201);
}

// A page with one heading below the title: too few for the TOC by default.
async function createShortPageWithAttachment(prefix: string) {
  const api = await loggedIn();
  const slug = `${prefix}-${Date.now()}`;
  const id = await createPageViaApi(api, {
    title: 'Panel Page',
    slug,
    content: '# Panel Page\n\n## Only Section\n\nText.\n',
  });
  await uploadAttachment(api, id);
  await api.dispose();
  return slug;
}

test.afterEach(async () => {
  await setAlwaysShowToc(false);
});

test('attachments keep the side panel open when the TOC is hidden', async ({ page }) => {
  const slug = await createShortPageWithAttachment('panel-attachments-only');

  await new LoginPage(page).loginViaApi(user, password);
  await new ViewPage(page).expectUserLoggedIn();
  await page.goto(toAppPath(`/${slug}`));
  await expect(page.locator('article>h1')).toHaveText('Panel Page');

  const panel = page.getByTestId('toc-side-panel');
  await expect(panel).toBeVisible();
  // Titled after the attachments, no headings and no second section header.
  await expect(panel.getByText('On this page')).toHaveCount(0);
  await expect(panel.getByText('Attached Media')).toHaveCount(1);
  await expect(panel.locator('[data-testid^="toc-entry-"]')).toHaveCount(0);

  const link = page.getByTestId(`toc-download-${fileName}`);
  await expect(link).toHaveAttribute('download', fileName);
  await expect(link).toHaveAttribute('title', fileName);
  await expect(link).toHaveAttribute('href', new RegExp(`/assets/.+/${fileName}$`));
});

test('the side panel lists headings first, then the attachments under their own header', async ({
  page,
}) => {
  const slug = await createShortPageWithAttachment('panel-both');
  await setAlwaysShowToc(true);

  await new LoginPage(page).loginViaApi(user, password);
  await new ViewPage(page).expectUserLoggedIn();
  await page.goto(toAppPath(`/${slug}`));
  await expect(page.locator('article>h1')).toHaveText('Panel Page');

  const panel = page.getByTestId('toc-side-panel');
  await expect(panel.getByText('On this page')).toHaveCount(1);
  const list = page.getByTestId('toc-side-panel-list');
  // Order: headings, the "Attached Media" header, then the download.
  await expect(list.locator('li')).toHaveText([
    'Panel Page',
    'Only Section',
    'Attached Media',
    fileName,
  ]);
  await expect(page.getByTestId('toc-entry-only-section')).toBeVisible();
});

test('the dropdown below the panel breakpoint lists the same sections', async ({ page }) => {
  const slug = await createShortPageWithAttachment('panel-dropdown');
  await setAlwaysShowToc(true);

  await page.setViewportSize({ width: 1024, height: 800 });
  await new LoginPage(page).loginViaApi(user, password);
  await new ViewPage(page).expectUserLoggedIn();
  await page.goto(toAppPath(`/${slug}`));
  await expect(page.locator('article>h1')).toHaveText('Panel Page');
  await expect(page.getByTestId('toc-side-panel')).toBeHidden();

  await page.getByRole('button', { name: 'On this page' }).click();
  const menu = page.getByRole('menu');
  await expect(menu.getByText('Attached Media')).toBeVisible();
  await expect(menu.getByTestId(`toc-dropdown-download-${fileName}`)).toHaveAttribute(
    'download',
    fileName,
  );

  await menu.getByTestId('toc-dropdown-entry-only-section').click();
  await expect(page).toHaveURL(/#only-section$/);
});

// Two highlights, the second one far enough down that it starts off screen.
// `==` inside code never becomes a highlight. Without the heading the page has
// no headings at all, so no TOC whatever "always show TOC" is set to.
async function createHighlightsPage(
  prefix: string,
  { withAttachment = false, withHeading = true } = {},
) {
  const api = await loggedIn();
  const slug = `${prefix}-${Date.now()}`;
  const filler = Array.from({ length: 60 }, (_, i) => `Filler paragraph ${i}.`).join('\n\n');
  const id = await createPageViaApi(api, {
    title: 'Highlights Page',
    slug,
    content: [
      ...(withHeading ? ['# Highlights Page'] : []),
      'Intro with ==first mark== and `==not a mark==` code.',
      filler,
      'Near the end, a =r=second mark== in red.',
    ].join('\n\n'),
  });
  if (withAttachment) await uploadAttachment(api, id);
  await api.dispose();
  return slug;
}

test('a page with only highlights shows the panel titled "Highlights"', async ({ page }) => {
  const slug = await createHighlightsPage('panel-highlights-only', { withHeading: false });

  await new LoginPage(page).loginViaApi(user, password);
  await new ViewPage(page).expectUserLoggedIn();
  await page.goto(toAppPath(`/${slug}`));
  await expect(page.locator('article mark', { hasText: 'first mark' })).toBeVisible();

  const panel = page.getByTestId('toc-side-panel');
  await expect(panel).toBeVisible();
  await expect(panel.getByText('Highlights', { exact: true })).toHaveCount(1);
  await expect(panel.getByText('On this page')).toHaveCount(0);
  await expect(page.getByTestId('toc-side-panel-list').locator('li')).toHaveText([
    'first mark',
    'second mark',
  ]);

  const urlBefore = page.url();
  const mark = page.locator('article mark', { hasText: 'second mark' });
  await expect(mark).not.toBeInViewport();
  await page.getByTestId('toc-highlight-1').click();
  await expect(mark).toBeInViewport();
  expect(page.url()).toBe(urlBefore);
});

test('the side panel lists headings, then highlights, then attachments', async ({ page }) => {
  const slug = await createHighlightsPage('panel-all-sections', { withAttachment: true });
  await setAlwaysShowToc(true);

  await new LoginPage(page).loginViaApi(user, password);
  await new ViewPage(page).expectUserLoggedIn();
  await page.goto(toAppPath(`/${slug}`));
  await expect(page.locator('article>h1')).toHaveText('Highlights Page');

  await expect(page.getByTestId('toc-side-panel-list').locator('li')).toHaveText([
    'Highlights Page',
    'Highlights',
    'first mark',
    'second mark',
    'Attached Media',
    fileName,
  ]);
});

const highlightLabels = (page: Page) => page.locator('[data-testid^="toc-highlight-"]');

async function createPage(prefix: string, title: string, content: string) {
  const api = await loggedIn();
  const slug = `${prefix}-${Date.now()}`;
  await createPageViaApi(api, { title, slug, content });
  await api.dispose();
  return slug;
}

test('the highlights follow in-app navigation between pages', async ({ page }) => {
  const target = await createPage('panel-nav-target', 'Nav Target', 'Only ==target mark== here.');
  const source = await createPage(
    'panel-nav-source',
    'Nav Source',
    `==source one== and ==source two==, then [go](/${target}).`,
  );
  const plain = await createPage('panel-nav-plain', 'Nav Plain', 'No highlights.');

  await new LoginPage(page).loginViaApi(user, password);
  await new ViewPage(page).expectUserLoggedIn();
  await page.goto(toAppPath(`/${source}`));
  await expect(highlightLabels(page)).toHaveText(['source one', 'source two']);

  await page.locator('article a', { hasText: 'go' }).click();
  await expect(page).toHaveURL(new RegExp(`/${target}$`));
  await expect(highlightLabels(page)).toHaveText(['target mark']);

  await page.goBack();
  await expect(highlightLabels(page)).toHaveText(['source one', 'source two']);

  // A page without highlights (and too few headings) has no panel at all.
  await page.evaluate(
    (path) => {
      window.history.pushState({}, '', path);
      window.dispatchEvent(new PopStateEvent('popstate'));
    },
    toAppPath(`/${plain}`),
  );
  await expect(page.locator('article')).toContainText('No highlights.');
  await expect(page.getByTestId('toc-side-panel')).toHaveCount(0);
});

test('a highlight added in the editor shows up after saving', async ({ page }) => {
  const slug = await createPage('panel-edit', 'Edit Highlights', 'Start ==before==.');

  await new LoginPage(page).loginViaApi(user, password);
  const viewPage = new ViewPage(page);
  await viewPage.expectUserLoggedIn();
  await page.goto(toAppPath(`/${slug}`));
  await expect(highlightLabels(page)).toHaveText(['before']);

  await viewPage.clickEditPageButton();
  const editPage = new EditPage(page);
  await page.locator('.cm-editor').click();
  await page.keyboard.press('ControlOrMeta+End');
  await editPage.writeContent('\n\nThen ==after==.');
  await editPage.savePage();
  await editPage.closeEditor();

  await expect(highlightLabels(page)).toHaveText(['before', 'after']);
});

test('clicking highlights after async content (math, mermaid, image) hits the right mark', async ({
  page,
}) => {
  const filler = Array.from({ length: 40 }, (_, i) => `Filler ${i}.`).join('\n\n');
  const longText = 'very long highlight '.repeat(20).trim();
  const slug = await createPage(
    'panel-async',
    'Async Highlights',
    [
      '==top mark==',
      '$$\n\\int_0^1 x^2 \\, dx\n$$',
      '```mermaid\ngraph TD; A-->B; B-->C; C-->D;\n```',
      '![img](https://example.invalid/missing.png)',
      filler,
      `==${longText}==`,
      filler,
      ':::collapsed Closed block\nInside ==hidden mark==\n:::',
      '==bottom mark==',
    ].join('\n\n'),
  );

  await new LoginPage(page).loginViaApi(user, password);
  await new ViewPage(page).expectUserLoggedIn();
  await page.goto(toAppPath(`/${slug}`));
  await expect(page.locator('article svg').first()).toBeVisible();
  await expect(highlightLabels(page)).toHaveText([
    'top mark',
    longText,
    'hidden mark',
    'bottom mark',
  ]);

  // Long labels are truncated with the full text in the tooltip.
  const longEntry = page.getByTestId('toc-highlight-1');
  await expect(longEntry).toHaveAttribute('title', longText);
  const box = await longEntry.boundingBox();
  const panelBox = await page.getByTestId('toc-side-panel').boundingBox();
  expect(box!.width).toBeLessThanOrEqual(panelBox!.width);

  await page.getByTestId('toc-highlight-3').click();
  await expect(page.locator('article mark', { hasText: 'bottom mark' })).toBeInViewport();

  await page.getByTestId('toc-highlight-1').click();
  await expect(page.locator('article mark', { hasText: 'very long' })).toBeInViewport();

  // A mark inside a closed collapsible: the click opens it and scrolls the
  // mark into view, without errors or a URL change.
  const errors: string[] = [];
  page.on('pageerror', (e) => errors.push(e.message));
  const urlBefore = page.url();
  const details = page.locator('article details', { hasText: 'hidden mark' });
  await expect(details).not.toHaveAttribute('open');
  await page.getByTestId('toc-highlight-2').click();
  await expect(details).toHaveAttribute('open');
  await expect(page.locator('article mark', { hasText: 'hidden mark' })).toBeInViewport();
  expect(page.url()).toBe(urlBefore);
  expect(errors).toEqual([]);
});

test('the dropdown below the panel breakpoint lists the highlights and scrolls to them', async ({
  page,
}) => {
  const slug = await createHighlightsPage('panel-highlights-dropdown', { withHeading: false });

  await page.setViewportSize({ width: 1024, height: 800 });
  await new LoginPage(page).loginViaApi(user, password);
  await new ViewPage(page).expectUserLoggedIn();
  await page.goto(toAppPath(`/${slug}`));
  await expect(page.locator('article mark', { hasText: 'first mark' })).toBeVisible();
  await expect(page.getByTestId('toc-side-panel')).toBeHidden();

  await page.getByRole('button', { name: 'Highlights' }).click();
  const menu = page.getByRole('menu');
  await expect(menu.locator('[data-testid^="toc-dropdown-highlight-"]')).toHaveText([
    'first mark',
    'second mark',
  ]);

  const mark = page.locator('article mark', { hasText: 'second mark' });
  await expect(mark).not.toBeInViewport();
  await menu.getByTestId('toc-dropdown-highlight-1').click();
  await expect(mark).toBeInViewport();
});
