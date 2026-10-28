import { APIRequestContext, expect, request, test } from '@playwright/test';
import { createPageViaApi, csrfHeader, loggedIn } from '../helpers/apiContext';

const baseURL = process.env.E2E_BASE_URL || 'http://localhost:8080';

// Public wikis serve page routes with title, description and Open Graph tags
// in the page source, so search engines and link previews see them without
// running the SPA (#1292). The E2E server runs without --public-url, so there
// is no canonical link.
test('public page source carries title, description and Open Graph tags', async () => {
  const admin = await loggedIn();
  const anonymous = await request.newContext({ baseURL });
  const slug = `seo-meta-${Date.now()}`;
  const title = `SEO Meta ${slug}`;
  await createPageViaApi(admin, {
    title,
    slug,
    content: '# Heading\n\nThe first paragraph & more.',
  });
  const siteName = (await (await admin.get('/api/branding')).json()).siteName as string;

  const setPublicAccess = async (enabled: boolean) => {
    const resp = await admin.put('/api/admin/settings/public-access', {
      headers: await csrfHeader(admin),
      data: { enabled },
    });
    expect(resp.ok(), `public access ${enabled}`).toBeTruthy();
  };
  const pageSource = async (ctx: APIRequestContext) => {
    const resp = await ctx.get(`/${slug}`);
    expect(resp.status()).toBe(200);
    return { html: await resp.text(), cacheControl: resp.headers()['cache-control'] };
  };

  try {
    await setPublicAccess(true);
    const { html, cacheControl } = await pageSource(anonymous);
    // Same format the SPA sets as document.title (documentTitle.ts).
    expect(html).toContain(`<title>${title} - ${siteName}</title>`);
    expect(html).toContain(
      '<meta name="description" content="Heading The first paragraph &amp; more.">',
    );
    expect(html).toContain('<meta property="og:type" content="website">');
    expect(html).toContain(`<meta property="og:title" content="${title}">`);
    expect(html).toContain(`<meta property="og:site_name" content="${siteName}">`);
    expect(html).not.toContain('rel="canonical"');
    expect(html).not.toContain('og:url');
    expect(cacheControl).toBe('no-cache');

    await setPublicAccess(false);
    const privateSource = await pageSource(anonymous);
    expect(privateSource.html).not.toContain('og:title');
    expect(privateSource.html).not.toContain('name="description"');
    expect(privateSource.cacheControl).toBeUndefined();
  } finally {
    await setPublicAccess(false);
    await anonymous.dispose();
    await admin.dispose();
  }
});
