import test, { expect } from '@playwright/test';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

// A 1x1 transparent PNG. Its size is irrelevant to this test — only the
// vertical margin Tailwind Typography's `prose img` rule puts around it
// matters, and a data: URI needs no asset upload to render.
const TINY_PNG =
  'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=';

function buildContent(): string {
  return [
    'Paragraph 1',
    'Paragraph 2',
    'Paragraph 3',
    `![Badge](${TINY_PNG}) Test 1 badge TODO`,
    `![Badge](${TINY_PNG}) Test 2 badge TODO`,
  ].join('\n\n');
}

async function createSpacingPage(
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
        throw new Error('Missing CSRF token cookie for image-spacing test setup');
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
    { pageTitle: title, pageSlug: slug, content: buildContent() },
  );

  const viewPage = new ViewPage(page);
  await viewPage.goto(`/${created.path}`);
  return viewPage;
}

test.describe('Image + text paragraph spacing', () => {
  test.beforeEach(async ({ page }) => {
    const loginPage = new LoginPage(page);
    await loginPage.goto();
    await loginPage.login(user, password);
    const viewPage = new ViewPage(page);
    await viewPage.expectUserLoggedIn();
  });

  test.afterEach(async ({ page }) => {
    const viewPage = new ViewPage(page);
    await viewPage.logout();
  });

  // Regression for #1524: Tailwind Typography's `prose img` rule gives images
  // a 2em top/bottom margin sized for a standalone block-level figure. Since
  // #1471 made Markdown images `inline-block` so trailing text stays on the
  // same line, that margin doesn't collapse with anything on an inline-block
  // box — it just inflates the line box's height. A single-line "![img] text"
  // paragraph ended up ~3.5x taller than a plain text paragraph (90px vs
  // 26px at the default font size), which is what reads as "too much
  // vertical space" around image+text lines, even though the margin
  // *between* the `<p>` elements themselves is unaffected.
  test('image+text paragraph height matches plain text paragraph height', async ({ page }) => {
    await createSpacingPage(page, `Image Spacing Page ${Date.now()}`);

    const paragraphs = page.locator('article.page-viewer__content > p');
    await expect(paragraphs).toHaveCount(5);
    await expect(paragraphs.nth(3).locator('img')).toBeVisible();
    await expect(paragraphs.nth(4).locator('img')).toBeVisible();

    const heights = await paragraphs.evaluateAll((els) =>
      els.map((el) => el.getBoundingClientRect().height),
    );

    const textParagraphHeight = heights[0];
    const imageParagraphHeight = heights[3];

    expect(textParagraphHeight).toBeGreaterThan(0);
    expect(Math.abs(imageParagraphHeight - textParagraphHeight)).toBeLessThan(6);
  });
});
