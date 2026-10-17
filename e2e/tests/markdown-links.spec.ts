import test, { expect } from '@playwright/test';
import { createPage } from '../helpers/api';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

test.describe('Markdown web link navigation', () => {
  test.beforeEach(async ({ page }) => {
    await new LoginPage(page).goto();
    await new LoginPage(page).login(user, password);
    await new ViewPage(page).expectUserLoggedIn();
  });

  test.afterEach(async ({ page }) => {
    await new ViewPage(page).logout();
  });

  for (const mode of ['viewer', 'editor'] as const) {
    for (const form of ['absolute', 'protocol-relative'] as const) {
      test(`${mode} follows ${form} same-origin links in the current tab`, async ({
        page,
        context,
      }) => {
        const stamp = Date.now();
        const sourceSlug = `markdown-links-source-${stamp}`;
        const targetSlug = `markdown-links-target-${stamp}`;
        const origin = new URL(page.url()).origin;
        const destination = `${origin}/${targetSlug}?mode=read&query=a%2Fb#heading`;
        const href = form === 'absolute' ? destination : destination.replace(/^https?:/, '');
        await createPage(page, {
          title: `Markdown target ${stamp}`,
          slug: targetSlug,
          content: `# Markdown target ${stamp}\n\n## Heading`,
        });
        await createPage(page, {
          title: `Markdown source ${stamp}`,
          slug: sourceSlug,
          content: `[Same origin](${href})\n\n[External](https://example.com/leafwiki-link-test)\n\n[Asset](assets/test.txt)\n\n[Relative](/${targetSlug})`,
        });
        const treeResponse = page.waitForResponse((response) =>
          response.url().endsWith('/api/tree'),
        );
        await page.reload();
        await (await treeResponse).finished();
        const viewPage = new ViewPage(page);
        await viewPage.goto(`/${sourceSlug}`);
        if (mode === 'editor') await viewPage.clickEditPageButton();

        const link = page.getByRole('link', { name: 'Same origin', exact: true });
        await expect(link).toHaveAttribute('href', href);
        await expect(link).not.toHaveAttribute('target');
        const asset = page.getByRole('link', { name: 'Asset', exact: true });
        await expect(asset).toHaveAttribute('target', '_blank');
        await expect(asset).toHaveAttribute('rel', 'noopener noreferrer');
        await expect(page.getByRole('link', { name: 'Relative', exact: true })).not.toHaveAttribute(
          'target',
        );

        await context.route('https://example.com/leafwiki-link-test', (route) =>
          route.fulfill({ contentType: 'text/html', body: '<title>External fixture</title>' }),
        );
        const external = page.getByRole('link', { name: 'External', exact: true });
        await expect(external).toHaveAttribute('target', '_blank');
        await expect(external).toHaveAttribute('rel', 'noopener noreferrer');
        const popupPromise = page.waitForEvent('popup');
        await external.click();
        const popup = await popupPromise;
        await popup.waitForLoadState();
        expect(await popup.evaluate(() => window.opener)).toBeNull();
        expect(await popup.evaluate(() => document.referrer)).toBe('');
        await popup.close();

        const navigation = page.waitForRequest(
          (request) => request.isNavigationRequest() && request.url().includes(targetSlug),
        );
        await link.focus();
        await page.keyboard.press('Enter');
        await navigation;
        await expect(page).toHaveURL(destination);
        await expect(page.locator('article > h1')).toHaveText(`Markdown target ${stamp}`);
        expect(context.pages()).toHaveLength(1);
      });
    }
  }
});
