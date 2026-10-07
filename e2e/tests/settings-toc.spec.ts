import { expect, test } from '@playwright/test';
import { createPageViaApi, csrfHeader, loggedIn } from '../helpers/apiContext';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';
import { toAppPath } from '../pages/appPath';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

async function setAlwaysShowToc(alwaysShow: boolean) {
  const api = await loggedIn();
  const resp = await api.put('/api/admin/settings/toc-display', {
    headers: await csrfHeader(api),
    data: { alwaysShow },
  });
  expect(resp.status()).toBe(200);
  await api.dispose();
}

// Instance-wide setting: always reset it so other specs see the default.
test.afterEach(async () => {
  await setAlwaysShowToc(false);
});

test('"always show TOC" shows the TOC for a page with only a few headings', async ({ page }) => {
  const api = await loggedIn();
  const slug = `toc-always-${Date.now()}`;
  await createPageViaApi(api, {
    title: 'Short Page',
    slug,
    content: '# Short Page\n\n## Only Section\n\nText.\n',
  });
  await api.dispose();

  const loginPage = new LoginPage(page);
  const viewPage = new ViewPage(page);
  await loginPage.goto();
  await loginPage.login(user, password);
  await viewPage.expectUserLoggedIn();

  await page.goto(toAppPath(`/${slug}`));
  await expect(page.locator('article>h1')).toHaveText('Short Page');
  await expect(page.getByTestId('toc-side-panel')).toHaveCount(0);

  await setAlwaysShowToc(true);
  await page.reload();
  await expect(page.locator('article>h1')).toHaveText('Short Page');
  await expect(page.getByTestId('toc-side-panel')).toBeVisible();

  await setAlwaysShowToc(false);
  await page.reload();
  await expect(page.locator('article>h1')).toHaveText('Short Page');
  await expect(page.getByTestId('toc-side-panel')).toHaveCount(0);
});

test('only admins can change the TOC setting', async () => {
  const admin = await loggedIn();
  const username = `toceditor${Date.now()}`;
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
  const resp = await editor.put('/api/admin/settings/toc-display', {
    headers: await csrfHeader(editor),
    data: { alwaysShow: true },
  });
  expect(resp.status()).toBe(403);

  await editor.dispose();
  await admin.delete(`/api/users/${editorId}`, { headers: await csrfHeader(admin) });
  await admin.dispose();
});
