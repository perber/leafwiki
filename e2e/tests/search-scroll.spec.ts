import { expect, test } from '@playwright/test';
import { createPageViaApi, loggedIn } from '../helpers/apiContext';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';
import { toAppPath } from '../pages/appPath';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

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
  await loginPage.goto();
  await loginPage.login(user, password);
  await new ViewPage(page).expectUserLoggedIn();

  await page.goto(toAppPath(`/${slug}?q=${needle}`));
  await expect(page.locator('article>h1')).toHaveText('Long Page');

  const mark = page.locator('mark.search-query-highlight');
  await expect(mark).toHaveText(needle);
  await expect(mark).toBeInViewport();
  await expect
    .poll(() => page.evaluate(() => document.getElementById('scroll-container')?.scrollTop ?? 0))
    .toBeGreaterThan(0);
});
