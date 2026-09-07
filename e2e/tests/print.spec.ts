import test, { expect } from '@playwright/test';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

// Long enough to span several printed pages so the browser has to paginate the
// article — that is what surfaced the clipping bug in #1531.
function buildLongContent(): string {
  const paragraph = Array.from(
    { length: 6 },
    () =>
      'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod ' +
      'tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam.',
  ).join(' ');

  const blocks: string[] = [];
  for (let i = 1; i <= 40; i++) {
    blocks.push(`## Section ${i}`);
    blocks.push(paragraph);
    blocks.push(paragraph);
  }
  return blocks.join('\n\n');
}

async function createLongPage(
  page: import('@playwright/test').Page,
  title: string,
): Promise<ViewPage> {
  const slug = title
    .toLowerCase()
    .replace(/\s+/g, '-')
    .replace(/[^\w-]/g, '');

  const created = await page.evaluate(
    async ({ pageTitle, pageSlug, content }) => {
      function getCsrfTokenFromCookie(): string | null {
        const hostMatch =
          document.cookie.match(/(?:^|;\s*)__Host-leafwiki_csrf=([^;]+)/) ??
          document.cookie.match(/(?:^|;\s*)leafwiki_csrf=([^;]+)/);

        if (!hostMatch) return null;

        try {
          return decodeURIComponent(hostMatch[1]);
        } catch {
          return hostMatch[1];
        }
      }

      const csrfToken = getCsrfTokenFromCookie();
      if (!csrfToken) {
        throw new Error('Missing CSRF token cookie for print test setup');
      }

      const createResponse = await fetch('/api/pages', {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': csrfToken,
        },
        body: JSON.stringify({
          parentId: null,
          title: pageTitle,
          slug: pageSlug,
          kind: 'page',
        }),
      });

      if (!createResponse.ok) {
        throw new Error(`Failed to create page ${pageTitle}: ${createResponse.status}`);
      }

      const createdPage = (await createResponse.json()) as {
        id: string;
        title: string;
        slug: string;
        path: string;
        version: string;
      };

      const updateResponse = await fetch(`/api/pages/${createdPage.id}`, {
        method: 'PUT',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': csrfToken,
        },
        body: JSON.stringify({
          version: createdPage.version,
          title: createdPage.title,
          slug: createdPage.slug,
          content,
        }),
      });

      if (!updateResponse.ok) {
        throw new Error(`Failed to update page ${createdPage.path}: ${updateResponse.status}`);
      }

      const updatedPage = (await updateResponse.json()) as typeof createdPage;
      return { path: updatedPage.path };
    },
    { pageTitle: title, pageSlug: slug, content: buildLongContent() },
  );

  const viewPage = new ViewPage(page);
  await viewPage.goto(`/${created.path}`);
  return viewPage;
}

test.describe('Print layout', () => {
  test.beforeEach(async ({ page }) => {
    const loginPage = new LoginPage(page);
    await loginPage.goto();
    await loginPage.login(user, password);
    const viewPage = new ViewPage(page);
    await viewPage.expectUserLoggedIn();
  });

  test.afterEach(async ({ page }) => {
    await page.emulateMedia({ media: null });
    const viewPage = new ViewPage(page);
    await viewPage.logout();
  });

  // Regression for #1531: scroll/clip containers around the article must not
  // survive into print. Chromium refuses to fragment a box whose `overflow`
  // is not `visible`, so any that leaked through clipped text at the page
  // break instead of letting it flow onto the next page.
  test('scroll containers unclip the article for pagination', async ({ page }) => {
    await createLongPage(page, `Print Clipping Page ${Date.now()}`);
    await page.locator('article').waitFor({ state: 'visible' });

    await page.emulateMedia({ media: 'print' });

    const overflow = await page.evaluate(() => {
      const selectors = [
        '.app-layout__main-column',
        '#scroll-container',
        '.page-viewer',
        '.page-viewer__body',
        '.page-viewer__content',
      ];
      return selectors.map((selector) => {
        const el = document.querySelector(selector);
        if (!el) return { selector, found: false, overflowX: '', overflowY: '' };
        const style = getComputedStyle(el);
        return {
          selector,
          found: true,
          overflowX: style.overflowX,
          overflowY: style.overflowY,
        };
      });
    });

    for (const entry of overflow) {
      expect(entry.found, `${entry.selector} should be present`).toBe(true);
      expect(entry.overflowX, `${entry.selector} overflow-x in print`).toBe('visible');
      expect(entry.overflowY, `${entry.selector} overflow-y in print`).toBe('visible');
    }
  });

  // Regression for #1531: headings need break hygiene so they stay attached to
  // the text they introduce rather than being stranded at a page bottom.
  test('headings carry print break hints', async ({ page }) => {
    await createLongPage(page, `Print Break Hints Page ${Date.now()}`);
    await page.locator('article h2').first().waitFor({ state: 'visible' });

    await page.emulateMedia({ media: 'print' });

    const heading = await page.evaluate(() => {
      const el = document.querySelector('.page-viewer__content h2');
      if (!el) return null;
      const style = getComputedStyle(el);
      return { breakAfter: style.breakAfter, breakInside: style.breakInside };
    });

    expect(heading).not.toBeNull();
    expect(heading?.breakAfter).toBe('avoid');
    expect(heading?.breakInside).toBe('avoid');
  });

  // The side TOC pane is navigation chrome — it must not print alongside the
  // article (the in-flow inline TOC is already hidden for print).
  test('side TOC pane is hidden in print', async ({ page }) => {
    await createLongPage(page, `Print TOC Page ${Date.now()}`);
    await page.locator('article').waitFor({ state: 'visible' });

    await page.emulateMedia({ media: 'print' });

    const tocDisplay = await page.evaluate(() => {
      const el = document.querySelector('#app-toc-pane-root');
      if (!el) return 'missing';
      return getComputedStyle(el).display;
    });

    expect(tocDisplay === 'none' || tocDisplay === 'missing').toBe(true);
  });
});
