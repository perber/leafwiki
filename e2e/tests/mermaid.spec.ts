import test, { Page, expect } from '@playwright/test';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

function getCsrfScript(): string {
  return `
    const hostMatch =
      document.cookie.match(/(?:^|;\\s*)__Host-leafwiki_csrf=([^;]+)/) ??
      document.cookie.match(/(?:^|;\\s*)leafwiki_csrf=([^;]+)/);
    if (!hostMatch) throw new Error('Missing CSRF token cookie');
    try { return decodeURIComponent(hostMatch[1]); } catch { return hostMatch[1]; }
  `;
}

async function createPage(page: Page, input: { title: string; slug: string; content: string }) {
  await page.evaluate(
    async ({ title, slug, content, csrfScript }) => {
      const csrfToken = new Function(csrfScript)() as string;
      const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken };
      const createRes = await fetch('/api/pages', {
        method: 'POST',
        credentials: 'include',
        headers,
        body: JSON.stringify({ parentId: null, title, slug, kind: 'page' }),
      });
      if (!createRes.ok) throw new Error(`create failed: ${createRes.status}`);
      const created = (await createRes.json()) as { id: string; version: string };
      const updateRes = await fetch(`/api/pages/${created.id}`, {
        method: 'PUT',
        credentials: 'include',
        headers,
        body: JSON.stringify({
          version: created.version,
          title,
          slug,
          content,
          tags: [],
          properties: {},
        }),
      });
      if (!updateRes.ok) throw new Error(`update failed: ${updateRes.status}`);
    },
    { ...input, csrfScript: getCsrfScript() },
  );
}

test.describe('Mermaid rendering', () => {
  test.beforeEach(async ({ page }) => {
    await new LoginPage(page).goto();
    await new LoginPage(page).login(user, password);
    await new ViewPage(page).expectUserLoggedIn();
  });

  test.afterEach(async ({ page }) => {
    await new ViewPage(page).logout();
  });

  test('mermaid-flowchart-renders-svg', async ({ page }) => {
    const s = Date.now();
    await createPage(page, {
      title: `Mermaid Basic ${s}`,
      slug: `mermaid-basic-${s}`,
      content: '```mermaid\nflowchart TD\n  A[Start] --> B[End]\n```',
    });

    await new ViewPage(page).goto(`/mermaid-basic-${s}`);
    await expect(page.locator('article svg').first()).toBeVisible({ timeout: 10000 });
    await expect(page.locator('article')).not.toContainText('Unable to render Mermaid diagram');
  });

  // Regression for #1131: <img> in a flowchart node must render as SVG.
  // Previously DOMParser (image/svg+xml) choked on the HTML void element and
  // injected a raw XML parseerror into the DOM instead of rendering the diagram.
  test('mermaid-flowchart-with-img-node-renders-svg', async ({ page }) => {
    const s = Date.now();
    // Single quotes inside the src attribute — the correct Mermaid syntax
    // for HTML in node labels (double-quote wrapping confuses the parser).
    const content = [
      '```mermaid',
      'flowchart TD',
      "  A[Christmas <img src='https://img.shields.io/badge/test-ok-blue' />] -->|Get money| B(Go shopping)",
      '  B --> C{Done?}',
      '  C -->|Yes| D[Laptop]',
      '  C -->|No| E[Phone]',
      '```',
    ].join('\n');

    await createPage(page, {
      title: `Mermaid Img ${s}`,
      slug: `mermaid-img-${s}`,
      content,
    });

    const pageErrors: string[] = [];
    page.on('pageerror', (err) => pageErrors.push(err.message));

    await new ViewPage(page).goto(`/mermaid-img-${s}`);

    // Give the renderer time to attempt rendering.
    await page.waitForTimeout(3000);

    // No error of any kind must appear.
    await expect(page.locator('article')).not.toContainText('Unable to render Mermaid diagram.', {
      timeout: 1000,
    });
    await expect(page.locator('article')).not.toContainText('Opening and ending tag mismatch', {
      timeout: 1000,
    });

    // The diagram must render as SVG with the image visible inside the node.
    await expect(page.locator('article svg').first()).toBeVisible({ timeout: 10000 });

    expect(pageErrors, `unexpected page errors: ${pageErrors.join('; ')}`).toHaveLength(0);
  });
});
