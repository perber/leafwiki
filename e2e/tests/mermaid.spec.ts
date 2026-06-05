import test, { expect } from '@playwright/test';
import { createPage } from '../helpers/api';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

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

  // Regression for #1131: <img> in a flowchart node must render as SVG for
  // both single-quoted and double-quoted src attributes.
  // Previously DOMParser (image/svg+xml) choked on the HTML void element and
  // injected a raw XML parseerror into the DOM instead of rendering the diagram.
  for (const [label, imgTag] of [
    [
      'singlequote',
      "<img src='data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==' />",
    ],
    [
      'doublequote',
      '<img src="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==" />',
    ],
  ] as const) {
    test(`mermaid-flowchart-with-img-${label}-node-renders-svg`, async ({ page }) => {
      const s = Date.now();
      const content = [
        '```mermaid',
        'flowchart TD',
        `  A[Christmas ${imgTag}] -->|Get money| B(Go shopping)`,
        '  B --> C{Done?}',
        '  C -->|Yes| D[Laptop]',
        '  C -->|No| E[Phone]',
        '```',
      ].join('\n');

      await createPage(page, {
        title: `Mermaid Img ${label} ${s}`,
        slug: `mermaid-img-${label}-${s}`,
        content,
      });

      const pageErrors: string[] = [];
      page.on('pageerror', (err) => pageErrors.push(err.message));

      await new ViewPage(page).goto(`/mermaid-img-${label}-${s}`);

      // Ensure the article loaded before polling for diagram state.
      await expect(page.locator('article')).toBeVisible({ timeout: 10000 });

      // Wait until either the SVG renders or the error UI is shown.
      await expect
        .poll(
          async () => {
            const svgVisible = await page
              .locator('article svg')
              .first()
              .isVisible()
              .catch(() => false);
            const errorUiVisible = await page
              .getByText('Unable to render Mermaid diagram.')
              .isVisible()
              .catch(() => false);
            return svgVisible || errorUiVisible;
          },
          { timeout: 15000 },
        )
        .toBe(true);

      // No error of any kind must appear.
      await expect(page.locator('article')).not.toContainText('Unable to render Mermaid diagram.', {
        timeout: 1000,
      });
      await expect(page.locator('article')).not.toContainText('Opening and ending tag mismatch', {
        timeout: 1000,
      });

      // The <img> node must survive SVG parsing and be rendered inside the diagram.
      await expect(page.locator('article svg img').first()).toBeVisible({ timeout: 5000 });

      expect(pageErrors, `unexpected page errors: ${pageErrors.join('; ')}`).toHaveLength(0);
    });
  }
});
