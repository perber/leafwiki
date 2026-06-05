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

  // Reproduces the bug reported in #1131: a flowchart node with an <img> tag
  // causes the DOMParser (image/svg+xml) to fail and insert a raw XML
  // parseerror element into the DOM instead of showing the error UI.
  //
  // Expected behaviour: either SVG renders (img stripped by security level)
  // or the "Unable to render" error UI is shown — but NEVER raw XML error text.
  test('mermaid-flowchart-with-img-node-does-not-show-xml-parse-error', async ({ page }) => {
    const s = Date.now();
    const content = [
      '```mermaid',
      'flowchart TD',
      '  A["Christmas <img src=\\"https://img.shields.io/badge/test-ok-blue\\" />"] -->|Get money| B(Go shopping)',
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

    // The raw XML parser error must never appear in the page — this is the bug.
    await expect(page.locator('article')).not.toContainText('Opening and ending tag mismatch', {
      timeout: 1000,
    });
    await expect(page.locator('article')).not.toContainText(
      'This page contains the following errors',
      { timeout: 1000 },
    );

    // Outcome must be one of: SVG rendered, or the proper error UI shown.
    const svgVisible = await page
      .locator('article svg')
      .first()
      .isVisible()
      .catch(() => false);
    const errorUiVisible = await page
      .getByText('Unable to render Mermaid diagram.')
      .isVisible()
      .catch(() => false);

    expect(
      svgVisible || errorUiVisible,
      `Expected SVG or error UI, got neither. Page errors: ${pageErrors.join('; ')}`,
    ).toBe(true);

    // Record which outcome occurred so it's visible in the test report.
    if (svgVisible) {
      console.log('✓ Mermaid rendered SVG (img stripped by security level)');
    } else {
      console.log('✓ Mermaid showed error UI (img caused parse failure — fix needed)');
    }
  });
});
