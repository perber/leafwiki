import { APIRequestContext, expect, test } from '@playwright/test';
import { createPageViaApi, csrfHeader, loggedIn, setAlwaysShowToc } from '../helpers/apiContext';
import LoginPage from '../pages/LoginPage';
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
