import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import test, { expect } from '@playwright/test';
import ImporterPage from '../pages/ImporterPage';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';
const separateRootEnabled = process.env.E2E_ENABLE_SEPARATE_ROOT_DIR === '1';
const assertRootFiles = process.env.E2E_ASSERT_SEPARATE_ROOT_FILES === '1';
const dataDir = process.env.E2E_DATA_DIR ?? '';
const rootDir = process.env.E2E_ROOT_DIR ?? '';
const importMetadataZipPath = path.resolve(
  __dirname,
  '../../internal/importer/fixtures/import-metadata.zip',
);
const importMetadataZipFileName = 'import-metadata.zip';

async function createPageWithContent(
  page: import('@playwright/test').Page,
  input: { title: string; slug: string; content: string },
) {
  await page.evaluate(async ({ title, slug, content }) => {
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
      throw new Error('Missing CSRF token cookie for separate root-dir setup');
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
        title,
        slug,
        kind: 'page',
      }),
    });

    if (!createResponse.ok) {
      throw new Error(`Failed to create page ${slug}: ${createResponse.status}`);
    }

    const createdPage = (await createResponse.json()) as {
      id: string;
      title: string;
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
        slug,
        content,
      }),
    });

    if (!updateResponse.ok) {
      throw new Error(`Failed to update page ${slug}: ${updateResponse.status}`);
    }
  }, input);
}

function expectMarkdownInConfiguredRoot(slug: string, expectedContent: string) {
  expect(dataDir, 'E2E_DATA_DIR should be exported by the local E2E runner').not.toBe('');
  expect(rootDir, 'E2E_ROOT_DIR should be exported by the local E2E runner').not.toBe('');

  const rootFile = path.join(rootDir, `${slug}.md`);
  const defaultRootFile = path.join(dataDir, 'root', `${slug}.md`);

  expect(existsSync(rootFile), `${rootFile} should exist`).toBe(true);
  expect(readFileSync(rootFile, 'utf8')).toContain(expectedContent);
  expect(existsSync(defaultRootFile), `${defaultRootFile} should not exist`).toBe(false);
}

test.describe('Separate root dir', () => {
  test.skip(!separateRootEnabled, 'requires E2E_ENABLE_SEPARATE_ROOT_DIR=1');

  test.beforeEach(async ({ page }) => {
    if (assertRootFiles) {
      expect(dataDir, 'E2E_DATA_DIR should be exported by the local E2E runner').not.toBe('');
      expect(rootDir, 'E2E_ROOT_DIR should be exported by the local E2E runner').not.toBe('');
    }

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

  test('writes page CRUD and imported markdown to the configured root dir', async ({ page }) => {
    const pageSlug = 'separate-root-page';
    const pageContent = '# Separate Root Page\n\nSeparate root E2E content';

    await createPageWithContent(page, {
      title: 'Separate Root Page',
      slug: pageSlug,
      content: pageContent,
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${pageSlug}`);
    await expect(page.getByRole('heading', { name: 'Separate Root Page' })).toBeVisible();

    if (assertRootFiles) {
      expectMarkdownInConfiguredRoot(pageSlug, 'Separate root E2E content');
    }

    const importerPage = new ImporterPage(page);
    await importerPage.goto();
    await importerPage.clearImportPlanIfPresent();
    await importerPage.uploadZip(importMetadataZipPath, importMetadataZipFileName);
    await importerPage.createImportPlan();
    await importerPage.executeImportPlan();

    await viewPage.goto('/imported-metadata-page');
    await expect(page.getByRole('heading', { name: 'Imported Metadata Page' })).toBeVisible();

    if (assertRootFiles) {
      expectMarkdownInConfiguredRoot('imported-metadata-page', 'Imported Metadata Page');
    }
  });
});
