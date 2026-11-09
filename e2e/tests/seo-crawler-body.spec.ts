import { APIRequestContext, expect, request, test } from '@playwright/test';
import { createPageViaApi, csrfHeader, loggedIn } from '../helpers/apiContext';

const baseURL = process.env.E2E_BASE_URL || 'http://localhost:8080';
const googlebot = 'Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)';
const chrome =
  'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36';

async function setPublicAccess(admin: APIRequestContext, enabled: boolean) {
  const resp = await admin.put('/api/admin/settings/public-access', {
    headers: await csrfHeader(admin),
    data: { enabled },
  });
  expect(resp.ok(), `public access ${enabled}`).toBeTruthy();
}

// Search-engine crawlers get the rendered page inside #root of a public
// wiki's page source (#1292). The SPA mounts with createRoot, which replaces
// that markup, so a crawler that runs JavaScript still sees the page once.
test('crawler gets the rendered page in #root, the SPA shows it once', async ({ browser }) => {
  const admin = await loggedIn();
  const slug = `seo-crawler-${Date.now()}`;
  const title = `SEO Crawler ${slug}`;
  const paragraph = `Crawler body paragraph ${slug}.`;
  await createPageViaApi(admin, { title, slug, content: paragraph });

  const pageSource = async (userAgent: string) => {
    const ctx = await request.newContext({ baseURL, userAgent });
    try {
      const resp = await ctx.get(`/${slug}`);
      expect(resp.status()).toBe(200);
      return { html: await resp.text(), vary: resp.headers()['vary'] ?? '' };
    } finally {
      await ctx.dispose();
    }
  };

  try {
    await setPublicAccess(admin, true);

    const crawler = await pageSource(googlebot);
    expect(crawler.html).toContain(`<div id="root"><article><h1>${title}</h1><p>${paragraph}</p>`);
    expect(crawler.vary).toContain('User-Agent');

    const visitor = await pageSource(chrome);
    expect(visitor.html).toContain('<div id="root"></div>');
    expect(visitor.vary).toContain('User-Agent');

    const context = await browser.newContext({ baseURL, userAgent: googlebot });
    try {
      const page = await context.newPage();
      await page.goto(`/${slug}`);
      // The server's article is a direct child of #root; the SPA's sits
      // inside its layout, so this waits until the SPA has replaced it.
      await expect(page.locator('#root > article')).toHaveCount(0);
      await expect(page.getByText(paragraph, { exact: true })).toBeVisible();
      await expect(page.getByText(paragraph, { exact: true })).toHaveCount(1);
      // The server's <h1> is gone; the SPA shows the title at most once.
      expect(await page.locator('h1', { hasText: title }).count()).toBeLessThanOrEqual(1);
    } finally {
      await context.close();
    }

    await setPublicAccess(admin, false);
    const privateSource = await pageSource(googlebot);
    expect(privateSource.html).toContain('<div id="root"></div>');
    expect(privateSource.vary).not.toContain('User-Agent');
  } finally {
    await setPublicAccess(admin, false);
    await admin.dispose();
  }
});
