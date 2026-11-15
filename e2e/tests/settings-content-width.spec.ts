import { type Page, expect, test } from '@playwright/test';
import {
  createPageViaApi,
  csrfHeader,
  loggedIn,
  setAlwaysShowToc,
  setContentWidth,
} from '../helpers/apiContext';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';
import { toAppPath } from '../pages/appPath';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';
const readingColumn = 680;
const paragraph =
  'A long paragraph that wraps over several lines so its box fills the available width. '.repeat(8);

// Instance-wide setting: always reset it so other specs see the default.
test.afterEach(async () => {
  await setContentWidth('default');
  await setAlwaysShowToc(false);
});

async function createLongPage() {
  const api = await loggedIn();
  const slug = `content-width-${Date.now()}`;
  await createPageViaApi(api, {
    title: 'Content Width',
    slug,
    content: `# Content Width\n\n${paragraph}\n\n${paragraph}\n`,
  });
  await api.dispose();
  return slug;
}

// Content boxes (inside padding) of the page view's content column, its
// first paragraph and the two header rows (subheader and metadata bar) whose
// text must line up with it. The header rows carry the page gutter as their
// own padding, so their border boxes start further left by design.
async function measureViewer(page: Page) {
  // The TOC pane and the subheader's reserved padding animate their width
  // (200ms, index.css). Measuring while that runs puts the header rows and
  // the content at different points of the animation, so wait until no CSS
  // transition is running. getAnimations() flushes pending style changes, so
  // a transition started by the last click is already listed.
  await page.waitForFunction(() =>
    document
      .getAnimations()
      .every(
        (animation) => !(animation instanceof CSSTransition) || animation.playState !== 'running',
      ),
  );
  return page.evaluate(() => {
    const contentBox = (selector: string) => {
      const el = document.querySelector(selector);
      if (!el) throw new Error(`${selector} is not rendered`);
      const rect = el.getBoundingClientRect();
      const style = getComputedStyle(el);
      const left = parseFloat(style.paddingLeft);
      const right = parseFloat(style.paddingRight);
      return { x: rect.x + left, width: rect.width - left - right };
    };
    return {
      content: contentBox('.page-viewer__content'),
      paragraph: contentBox('.page-viewer__content p'),
      subheaderInner: contentBox('.page-viewer__subheader-inner'),
      metadataInner: contentBox('.page-viewer__metadata-bar-inner'),
    };
  });
}

test('"wide" lifts the 680px reading column in the page view, header rows stay aligned', async ({
  page,
}) => {
  const slug = await createLongPage();
  await page.setViewportSize({ width: 1280, height: 900 });
  const loginPage = new LoginPage(page);
  const viewPage = new ViewPage(page);
  await loginPage.loginViaApi(user, password);
  await viewPage.expectUserLoggedIn();

  await page.goto(toAppPath(`/${slug}`));
  await expect(page.locator('article>h1')).toHaveText('Content Width');
  await expect(page.locator('.page-viewer')).toHaveAttribute('data-content-width', 'default');
  const narrow = await measureViewer(page);
  expect(narrow.content.width).toBeLessThanOrEqual(readingColumn);
  expect(narrow.paragraph.width).toBeLessThanOrEqual(readingColumn);
  expect(Math.abs(narrow.subheaderInner.x - narrow.content.x)).toBeLessThan(3);

  await setContentWidth('wide');
  await page.reload();
  await expect(page.locator('article>h1')).toHaveText('Content Width');
  await expect(page.locator('.page-viewer')).toHaveAttribute('data-content-width', 'wide');
  const wide = await measureViewer(page);
  expect(wide.content.width).toBeGreaterThan(readingColumn);
  expect(wide.paragraph.width).toBeGreaterThan(readingColumn);
  for (const [name, row] of [
    ['subheader', wide.subheaderInner],
    ['metadata bar', wide.metadataInner],
  ] as const) {
    expect(row.width, `${name} row is wider than the reading column`).toBeGreaterThan(
      readingColumn,
    );
    expect(
      Math.abs(row.x - wide.content.x),
      `${name} row starts where the content starts`,
    ).toBeLessThan(3);
  }
});

test('"wide" keeps the 680px column in the editor preview', async ({ page }) => {
  const slug = await createLongPage();
  await setContentWidth('wide');
  // Wide enough that the preview pane itself exceeds the reading column.
  await page.setViewportSize({ width: 2200, height: 900 });
  const loginPage = new LoginPage(page);
  const viewPage = new ViewPage(page);
  await loginPage.loginViaApi(user, password);
  await viewPage.expectUserLoggedIn();

  await page.goto(toAppPath(`/e/${slug}`));
  const previewParagraph = page.locator('.markdown-editor__preview p').first();
  await expect(previewParagraph).toBeVisible();
  const preview = await page.locator('.markdown-editor__preview').boundingBox();
  const prose = await previewParagraph.boundingBox();
  expect(preview!.width, 'precondition: the preview pane is wider than the column').toBeGreaterThan(
    readingColumn,
  );
  expect(prose!.width).toBeLessThanOrEqual(readingColumn);
});

// With the TOC side panel open or collapsed, "wide" content must still line
// up with the header rows and never run under the panel.
for (const tocState of ['expanded', 'collapsed'] as const) {
  test(`"wide" stays aligned next to the ${tocState} TOC side panel`, async ({ page }) => {
    const slug = await createLongPage();
    await setContentWidth('wide');
    await setAlwaysShowToc(true);
    await page.setViewportSize({ width: 1600, height: 900 });
    const loginPage = new LoginPage(page);
    const viewPage = new ViewPage(page);
    await loginPage.loginViaApi(user, password);
    await viewPage.expectUserLoggedIn();

    await page.goto(toAppPath(`/${slug}`));
    await expect(page.locator('article>h1')).toHaveText('Content Width');
    const panel = page.getByTestId('toc-side-panel');
    await expect(panel).toBeVisible();
    if (tocState === 'collapsed') {
      await page.getByTestId('toc-side-panel-collapse').click();
      await expect(page.getByTestId('toc-side-panel-expand')).toBeVisible();
    } else if (await page.getByTestId('toc-side-panel-expand').isVisible()) {
      await page.getByTestId('toc-side-panel-expand').click();
    }

    const wide = await measureViewer(page);
    const panelBox = (await panel.boundingBox())!;
    expect(wide.paragraph.width).toBeGreaterThan(readingColumn);
    expect(
      wide.content.x + wide.content.width,
      'content ends before the TOC panel starts',
    ).toBeLessThanOrEqual(panelBox.x + 1);
    for (const [name, row] of [
      ['subheader', wide.subheaderInner],
      ['metadata bar', wide.metadataInner],
    ] as const) {
      expect(
        Math.abs(row.x - wide.content.x),
        `${name} row starts where the content starts`,
      ).toBeLessThan(3);
    }
  });
}

test('"wide" keeps the 680px column in the page history preview', async ({ page }) => {
  const slug = await createLongPage();
  await setContentWidth('wide');
  await page.setViewportSize({ width: 2200, height: 900 });
  const loginPage = new LoginPage(page);
  const viewPage = new ViewPage(page);
  await loginPage.loginViaApi(user, password);
  await viewPage.expectUserLoggedIn();

  await page.goto(toAppPath(`/history/${slug}`));
  const content = page.getByTestId('page-history-page-content');
  // The history page opens the previous revision; preview the current one,
  // which holds the long paragraph.
  await page
    .locator('button[data-testid^="history-sidebar-revision-"]', {
      has: page.locator('[data-testid^="history-sidebar-revision-current-badge-"]'),
    })
    .click();
  const prose = content.locator('p').first();
  await expect(prose).toBeVisible();
  const contentBox = (await content.boundingBox())!;
  const proseBox = (await prose.boundingBox())!;
  expect(contentBox.width, 'precondition: the preview is wider than the column').toBeGreaterThan(
    readingColumn,
  );
  expect(proseBox.width).toBeLessThanOrEqual(readingColumn);
});

test('only admins can change the page width', async () => {
  const admin = await loggedIn();
  const username = `widtheditor${Date.now()}`;
  const created = await admin.post('/api/users', {
    headers: await csrfHeader(admin),
    data: {
      username,
      email: `${username}@example.com`,
      password: 'editor-password-123',
      role: 'editor',
    },
  });
  expect(created.status()).toBe(201);
  const editorId = (await created.json()).id;

  const editor = await loggedIn(username, 'editor-password-123');
  const resp = await editor.put('/api/admin/settings/content-width', {
    headers: await csrfHeader(editor),
    data: { contentWidth: 'wide' },
  });
  expect(resp.status()).toBe(403);

  await editor.dispose();
  await admin.delete(`/api/users/${editorId}`, { headers: await csrfHeader(admin) });
  await admin.dispose();
});
