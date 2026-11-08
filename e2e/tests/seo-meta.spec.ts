import { APIRequestContext, expect, request, test } from '@playwright/test';
import { createPageViaApi, csrfHeader, loggedIn } from '../helpers/apiContext';

const baseURL = process.env.E2E_BASE_URL || 'http://localhost:8080';

async function setPublicAccess(admin: APIRequestContext, enabled: boolean) {
  const resp = await admin.put('/api/admin/settings/public-access', {
    headers: await csrfHeader(admin),
    data: { enabled },
  });
  expect(resp.ok(), `public access ${enabled}`).toBeTruthy();
}

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

  const pageSource = async (ctx: APIRequestContext) => {
    const resp = await ctx.get(`/${slug}`);
    expect(resp.status()).toBe(200);
    return { html: await resp.text(), cacheControl: resp.headers()['cache-control'] };
  };

  try {
    await setPublicAccess(admin, true);
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

    await setPublicAccess(admin, false);
    const privateSource = await pageSource(anonymous);
    expect(privateSource.html).not.toContain('og:title');
    expect(privateSource.html).not.toContain('name="description"');
    expect(privateSource.cacheControl).toBeUndefined();
  } finally {
    await setPublicAccess(admin, false);
    await anonymous.dispose();
    await admin.dispose();
  }
});

// A permalink /p/<id>/<slug> gets the meta of the page the ID points at, even
// with a stale slug; the server's locator mirrors the SPA route /p/:id/:slug?,
// so the browser must land on that same page.
test('public permalink source carries the target page meta', async ({ browser }) => {
  const admin = await loggedIn();
  const anonymous = await request.newContext({ baseURL });
  const slug = `seo-permalink-${Date.now()}`;
  const title = `SEO Permalink ${slug}`;
  const id = await createPageViaApi(admin, { title, slug, content: 'Permalink body.' });
  const permalink = `/p/${encodeURIComponent(id)}/stale-slug`;

  try {
    await setPublicAccess(admin, true);
    const resp = await anonymous.get(permalink);
    expect(resp.status()).toBe(200);
    const html = await resp.text();
    expect(html).toContain(`<meta property="og:title" content="${title}">`);
    expect(html).toContain('<meta name="description" content="Permalink body.">');

    const context = await browser.newContext({ baseURL });
    try {
      const page = await context.newPage();
      await page.goto(permalink);
      await expect(page).toHaveURL(new RegExp(`/${slug}$`));
    } finally {
      await context.close();
    }
  } finally {
    await setPublicAccess(admin, false);
    await anonymous.dispose();
    await admin.dispose();
  }
});
