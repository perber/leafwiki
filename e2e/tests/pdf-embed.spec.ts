import { expect, test } from '@playwright/test';
import { createPageViaApi, csrfHeader, loggedIn } from '../helpers/apiContext';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';
import { toAppPath } from '../pages/appPath';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

const minimalPdf = `%PDF-1.4
1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj
2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj
3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]>>endobj
trailer<</Root 1 0 R>>
%%EOF`;

let slug: string;

test.beforeAll(async () => {
  const api = await loggedIn();
  slug = `pdf-embed-${Date.now()}`;
  const pageId = await createPageViaApi(api, { title: 'PDF Page', slug, content: 'placeholder' });
  const upload = await api.post(`/api/pages/${pageId}/assets`, {
    headers: await csrfHeader(api),
    multipart: {
      file: { name: 'manual.pdf', mimeType: 'application/pdf', buffer: Buffer.from(minimalPdf) },
    },
  });
  expect(upload.status()).toBe(201);
  const url = (await upload.json()).file as string;

  const current = await (await api.get(`/api/pages/${pageId}`)).json();
  const updated = await api.put(`/api/pages/${pageId}`, {
    headers: await csrfHeader(api),
    data: {
      version: current.version,
      title: 'PDF Page',
      slug,
      content: `# PDF Page\n\n![Manual](${url})\n`,
      tags: [],
      properties: {},
    },
  });
  expect(updated.ok()).toBeTruthy();
  await api.dispose();
});

async function openPdfPage(page: import('@playwright/test').Page) {
  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login(user, password);
  await new ViewPage(page).expectUserLoggedIn();
  await page.goto(toAppPath(`/${slug}`));
  await expect(page.locator('article>h1')).toHaveText('PDF Page');
}

test('a pdf asset is embedded as an iframe on desktop', async ({ page }) => {
  await openPdfPage(page);
  const frame = page.locator('.markdown-pdf-embed iframe');
  await expect(frame).toHaveCount(1);
  await expect(frame).toHaveAttribute('src', /manual\.pdf/);
  await expect(frame).toHaveAttribute('title', 'Manual');
});

test.describe('mobile', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('a pdf asset is offered as an open button instead of an iframe', async ({ page }) => {
    await openPdfPage(page);
    await expect(page.locator('.markdown-pdf-embed iframe')).toHaveCount(0);
    const link = page.locator('.markdown-pdf-embed a');
    await expect(link).toHaveAttribute('href', /manual\.pdf/);
    await expect(link).toHaveAttribute('target', '_blank');
  });
});
