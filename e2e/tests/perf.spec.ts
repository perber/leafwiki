import { expect, test } from '@playwright/test';
import LoginPage from '../pages/LoginPage';
import TreeView from '../pages/TreeView';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';
const shouldRunBenchmark = process.env.E2E_ENABLE_BENCHMARK === '1';
const seedCount = Number(process.env.E2E_BENCHMARK_PAGE_COUNT || '520');

type SeedPage = {
  title: string;
  slug: string;
};

type CreatedPage = SeedPage & {
  id: string;
};

type SeedResult = {
  createdPages: CreatedPage[];
  durationMs: number;
};

async function seedPages(page: import('@playwright/test').Page, pages: SeedPage[]) {
  const result = await page.evaluate(async (seedPagesInput) => {
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
      throw new Error('Missing CSRF token cookie for benchmark seed');
    }

    const startedAt = performance.now();
    const createdPages: CreatedPage[] = [];

    for (const seedPage of seedPagesInput) {
      const response = await fetch('/api/pages', {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': csrfToken,
        },
        body: JSON.stringify({
          parentId: null,
          title: seedPage.title,
          slug: seedPage.slug,
          kind: 'page',
        }),
      });

      if (!response.ok) {
        const bodyText = await response.text();
        throw new Error(`Failed to seed ${seedPage.slug}: ${response.status} ${bodyText}`);
      }

      const createdPage = (await response.json()) as { id: string; title: string; slug: string };
      createdPages.push({
        id: createdPage.id,
        title: seedPage.title,
        slug: seedPage.slug,
      });
    }

    return {
      createdPages,
      durationMs: performance.now() - startedAt,
    };
  }, pages);

  return result;
}

async function cleanupPages(page: import('@playwright/test').Page, createdPages: CreatedPage[]) {
  if (createdPages.length === 0) return;

  await page.evaluate(async (pagesToDelete) => {
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
      throw new Error('Missing CSRF token cookie for benchmark cleanup');
    }

    for (const createdPage of [...pagesToDelete].reverse()) {
      const response = await fetch(`/api/pages/${createdPage.id}?recursive=false`, {
        method: 'DELETE',
        credentials: 'include',
        headers: {
          'X-CSRF-Token': csrfToken,
        },
      });

      if (!response.ok) {
        const bodyText = await response.text();
        throw new Error(`Failed to delete ${createdPage.slug}: ${response.status} ${bodyText}`);
      }
    }
  }, createdPages);
}

async function measureTreeNavigation(
  page: import('@playwright/test').Page,
  targetTitle: string,
  targetSlug: string,
) {
  const treeView = new TreeView(page);
  const link = await treeView.findPageByTitle(targetTitle);

  await link.scrollIntoViewIfNeeded();

  const startedAt = Date.now();
  const requestPromise = page.waitForRequest((request) =>
    request.url().includes(`/api/pages/by-path?path=${encodeURIComponent(targetSlug)}`),
  );
  const responsePromise = page.waitForResponse(
    (response) =>
      response.url().includes(`/api/pages/by-path?path=${encodeURIComponent(targetSlug)}`) &&
      response.status() === 200,
  );

  await link.click();
  await requestPromise;
  const requestStartedMs = Date.now() - startedAt;
  await responsePromise;
  const responseMs = Date.now() - startedAt;
  const requestDurationMs = responseMs - requestStartedMs;
  await expect(page.locator('article > h1')).toHaveText(targetTitle);
  const totalMs = Date.now() - startedAt;
  const renderAfterResponseMs = totalMs - responseMs;

  return {
    totalMs,
    requestStartedMs,
    responseMs,
    requestDurationMs,
    renderAfterResponseMs,
  };
}

test.describe('Performance', () => {
  test.skip(!shouldRunBenchmark, 'Set E2E_ENABLE_BENCHMARK=1 to run the 500+ page benchmark.');

  test('tree-navigation-benchmark-500-pages', async ({ page }) => {
    test.setTimeout(10 * 60 * 1000);
    test.slow();

    const loginPage = new LoginPage(page);
    await loginPage.goto();
    await loginPage.login(user, password);

    const viewPage = new ViewPage(page);
    await viewPage.expectUserLoggedIn();

    const benchmarkPages = Array.from({ length: seedCount }, (_, index) => {
      const pageNumber = String(index + 1).padStart(4, '0');
      return {
        title: `Benchmark Page ${pageNumber}`,
        slug: `benchmark-page-${pageNumber}`,
      };
    });

    let createdPages: CreatedPage[] = [];

    try {
      const seedResult: SeedResult = await seedPages(page, benchmarkPages);
      createdPages = seedResult.createdPages;
      await page.reload();

      const treeView = new TreeView(page);
      await expect
        .poll(async () => treeView.getNumberOfTreeNodes(), {
          timeout: 60_000,
          message: `Expected at least ${seedCount} benchmark nodes to be visible in the tree`,
        })
        .toBeGreaterThanOrEqual(seedCount);

      const samples = [
        benchmarkPages[0],
        benchmarkPages[Math.floor(benchmarkPages.length / 2)],
        benchmarkPages[benchmarkPages.length - 1],
      ];

      const navigationDurationsMs: number[] = [];
      const requestStartedDurationsMs: number[] = [];
      const responseDurationsMs: number[] = [];
      const requestDurationsMs: number[] = [];
      const renderAfterResponseDurationsMs: number[] = [];
      for (const sample of samples) {
        const measurement = await measureTreeNavigation(page, sample.title, sample.slug);
        navigationDurationsMs.push(measurement.totalMs);
        requestStartedDurationsMs.push(measurement.requestStartedMs);
        responseDurationsMs.push(measurement.responseMs);
        requestDurationsMs.push(measurement.requestDurationMs);
        renderAfterResponseDurationsMs.push(measurement.renderAfterResponseMs);
      }

      const averageDurationMs =
        navigationDurationsMs.reduce((sum, duration) => sum + duration, 0) /
        navigationDurationsMs.length;
      const averageRequestStartedMs =
        requestStartedDurationsMs.reduce((sum, duration) => sum + duration, 0) /
        requestStartedDurationsMs.length;
      const averageResponseMs =
        responseDurationsMs.reduce((sum, duration) => sum + duration, 0) /
        responseDurationsMs.length;
      const averageRequestDurationMs =
        requestDurationsMs.reduce((sum, duration) => sum + duration, 0) / requestDurationsMs.length;
      const averageRenderAfterResponseMs =
        renderAfterResponseDurationsMs.reduce((sum, duration) => sum + duration, 0) /
        renderAfterResponseDurationsMs.length;

      console.log(
        JSON.stringify({
          benchmark: 'tree-navigation-benchmark-500-pages',
          seedCount,
          seedDurationMs: Math.round(seedResult.durationMs),
          navigationDurationsMs,
          requestStartedDurationsMs,
          responseDurationsMs,
          requestDurationsMs,
          renderAfterResponseDurationsMs,
          averageDurationMs: Math.round(averageDurationMs),
          averageRequestStartedMs: Math.round(averageRequestStartedMs),
          averageResponseMs: Math.round(averageResponseMs),
          averageRequestDurationMs: Math.round(averageRequestDurationMs),
          averageRenderAfterResponseMs: Math.round(averageRenderAfterResponseMs),
        }),
      );
    } finally {
      await cleanupPages(page, createdPages);
    }
  });
});
