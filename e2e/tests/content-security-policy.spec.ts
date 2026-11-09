import { Page, expect, test } from '@playwright/test';
import { createPage } from '../helpers/api';
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

type Violation = { directive: string; blockedURI: string; source: string };

// Init scripts are not subject to the page's CSP, so the listener is in place
// before the app's first script runs; the exposed function collects across
// navigations. The console catches what the event misses (e.g. a worker).
async function recordPolicyViolations(page: Page) {
  const violations: Violation[] = [];
  const consoleMessages: string[] = [];
  page.on('console', (msg) => {
    const text = msg.text();
    if (/content[- ]security[- ]policy|refused to (load|execute|apply|create|frame)/i.test(text)) {
      consoleMessages.push(text);
    }
  });
  await page.exposeFunction('__reportPolicyViolation', (v: Violation) => violations.push(v));
  await page.addInitScript(() => {
    document.addEventListener('securitypolicyviolation', (e) => {
      const report = (window as unknown as { __reportPolicyViolation: (v: Violation) => void })
        .__reportPolicyViolation;
      report({
        directive: e.effectiveDirective,
        blockedURI: e.blockedURI,
        source: `${e.sourceFile}:${e.lineNumber}`,
      });
    });
  });
  return () => {
    expect(violations, 'securitypolicyviolation events').toEqual([]);
    expect(consoleMessages, 'CSP console messages').toEqual([]);
  };
}

let pdfSlug: string;

test.beforeAll(async () => {
  const api = await loggedIn();
  pdfSlug = `csp-pdf-${Date.now()}`;
  const pageId = await createPageViaApi(api, { title: 'CSP PDF', slug: pdfSlug, content: 'x' });
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
      title: 'CSP PDF',
      slug: pdfSlug,
      content: `# CSP PDF\n\n![Manual](${url})\n`,
      tags: [],
      properties: {},
    },
  });
  expect(updated.ok()).toBeTruthy();
  await api.dispose();
});

test('the shell restricts scripts to its own origin', async ({ page }) => {
  const response = await page.goto(toAppPath('/login'));
  const policy = response?.headers()['content-security-policy'] ?? '';
  expect(policy).toContain("script-src 'self'");
  expect(policy).toContain("frame-ancestors 'self'");
});

test('login, page view, math, Mermaid, PDF embed and editor run without policy violations', async ({
  page,
}) => {
  const assertNoViolations = await recordPolicyViolations(page);

  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login(user, password);
  const viewPage = new ViewPage(page);
  await viewPage.expectUserLoggedIn();

  const s = Date.now();
  const slug = `csp-content-${s}`;
  await createPage(page, {
    title: `CSP Content ${s}`,
    slug,
    content: [
      'Inline math $a^2 + b^2 = c^2$.',
      '',
      '$$',
      '\\sum_{i=1}^{n} a_i',
      '$$',
      '',
      '```mermaid',
      'flowchart TD',
      '  A[Start] --> B[End]',
      '```',
      '',
      '```mermaid',
      '---',
      'config:',
      '  layout: elk',
      '---',
      'flowchart LR',
      '  C --> D',
      '```',
    ].join('\n'),
  });

  await viewPage.goto(`/${slug}`);
  const article = page.locator('article');
  await expect(article.locator('.katex-display')).toBeVisible();
  // Mermaid marks its root <svg>; nested and icon SVGs don't count.
  await expect(article.locator('svg[aria-roledescription]')).toHaveCount(2, { timeout: 15000 });
  await expect(article).not.toContainText('Unable to render Mermaid diagram');

  await viewPage.clickEditPageButton();
  await expect(page.locator('.cm-editor')).toBeVisible();

  await page.goto(toAppPath(`/${pdfSlug}`));
  await expect(page.locator('.markdown-pdf-embed iframe')).toHaveCount(1);

  assertNoViolations();
});
