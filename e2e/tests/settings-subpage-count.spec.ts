import { expect, test } from '@playwright/test';
import {
  createNodeViaApi,
  loggedIn,
  setPublicAccess,
  setShowSubpageCount,
} from '../helpers/apiContext';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';
import { toAppPath } from '../pages/appPath';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

// Instance-wide settings: always reset them so other specs see the defaults.
test.afterEach(async () => {
  await setShowSubpageCount(false);
  await setPublicAccess(false);
});

// A root section "Handbook" with a page and a sub-section; the sub-section
// holds one page. Handbook has no content, so its view is the section
// overview listing both children.
async function createHandbook() {
  const api = await loggedIn();
  const suffix = Date.now();
  const slug = `handbook-${suffix}`;
  const handbookId = await createNodeViaApi(api, {
    title: `Handbook ${suffix}`,
    slug,
    kind: 'section',
    parentId: null,
  });
  await createNodeViaApi(api, {
    title: 'Welcome',
    slug: 'welcome',
    kind: 'page',
    parentId: handbookId,
  });
  const teamsId = await createNodeViaApi(api, {
    title: 'Teams',
    slug: 'teams',
    kind: 'section',
    parentId: handbookId,
  });
  await createNodeViaApi(api, { title: 'Ops', slug: 'ops', kind: 'page', parentId: teamsId });
  await api.dispose();
  return { slug, title: `Handbook ${suffix}`, handbookId, teamsId };
}

test('an admin turns on the subpage count; tree and section overview show it', async ({ page }) => {
  const handbook = await createHandbook();
  await page.setViewportSize({ width: 1280, height: 900 });
  const loginPage = new LoginPage(page);
  const viewPage = new ViewPage(page);
  await loginPage.loginViaApi(user, password);
  await viewPage.expectUserLoggedIn();

  const treeCount = page.getByTestId(`tree-node-subpage-count-${handbook.handbookId}`);
  await page.goto(toAppPath(`/${handbook.slug}`));
  await expect(page.getByTestId(`tree-node-link-${handbook.handbookId}`)).toBeVisible();
  await expect(treeCount).toHaveCount(0);

  await page.goto(toAppPath('/settings/display'));
  const checkbox = page.getByTestId('display-subpage-count-checkbox');
  await expect(checkbox).not.toBeChecked();
  await checkbox.click();
  await expect(checkbox).toBeChecked();
  await expect(page.getByText('Subpage count setting saved')).toBeVisible();

  await page.goto(toAppPath(`/${handbook.slug}`));
  await expect(treeCount).toHaveText('2 2 subpages');
  await expect(page.getByTestId(`tree-node-link-${handbook.handbookId}`)).toHaveAccessibleName(
    `${handbook.title} 2 subpages`,
  );
  const overviewCount = page.getByTestId(`subpage-count-${handbook.teamsId}`);
  await expect(overviewCount).toHaveText('1 1 subpage');
  await expect(
    page
      .getByRole('navigation', { name: `Subpages of ${handbook.title}` })
      .getByRole('link', { name: 'Teams 1 subpage' }),
  ).toBeVisible();
});

// The page fixture has no session: it browses as an anonymous visitor.
test('anonymous visitors of a public wiki see the subpage count too', async ({ page }) => {
  const handbook = await createHandbook();
  await setPublicAccess(true);
  await setShowSubpageCount(true);

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(toAppPath(`/${handbook.slug}`));
  await expect(page.getByTestId(`tree-node-subpage-count-${handbook.handbookId}`)).toHaveText(
    '2 2 subpages',
  );
  await expect(page.getByTestId(`subpage-count-${handbook.teamsId}`)).toHaveText('1 1 subpage');
});
