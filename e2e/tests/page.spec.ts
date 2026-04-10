import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from '@playwright/test';
import AddPageDialog from '../pages/AddPageDialog';
import CopyPageDialog from '../pages/CopyPageDialog';
import CreatePageByPathDialog from '../pages/CreatePageByPathDialog';
import DeletePageDialog from '../pages/DeletePageDialog';
import EditPage from '../pages/EditPage';
import EditPageMetadataDialog from '../pages/EditPageMetadataDialog';
import LoginPage from '../pages/LoginPage';
import NotFoundPage from '../pages/NotFoundPage';
import TreeView from '../pages/TreeView';
import ViewPage from '../pages/ViewPage';
import { e2eBasePath, toAppPath } from '../pages/appPath';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

const currentDir = __dirname;
const markdownItSamplePath = join(currentDir, '..', 'assets', 'markdown-it-sample.md');

async function dispatchLayoutShortcut(
  page: import('@playwright/test').Page,
  eventInit: {
    key: string;
    code: string;
    ctrlKey?: boolean;
    altKey?: boolean;
    shiftKey?: boolean;
  },
) {
  await page.evaluate((keyboardEventInit) => {
    const target = document.activeElement instanceof HTMLElement ? document.activeElement : window;

    const event = new KeyboardEvent('keydown', {
      key: keyboardEventInit.key,
      code: keyboardEventInit.code,
      bubbles: true,
      cancelable: true,
      ctrlKey: keyboardEventInit.ctrlKey ?? false,
      altKey: keyboardEventInit.altKey ?? false,
      shiftKey: keyboardEventInit.shiftKey ?? false,
    });

    target.dispatchEvent(event);
  }, eventInit);
}

async function createPageAndOpenViewer(page: import('@playwright/test').Page, title: string) {
  const treeView = new TreeView(page);
  const curNodeCount = await treeView.getNumberOfTreeNodes();
  await treeView.clickRootAddButton();

  const addPageDialog = new AddPageDialog(page);
  await addPageDialog.fillTitle(title);
  await addPageDialog.submitWithoutRedirect();

  await treeView.expectNumberOfTreeNodes(curNodeCount + 1);
  await treeView.clickPageByTitle(title);

  const viewPage = new ViewPage(page);
  test.expect(await viewPage.getTitle()).toBe(title);

  return viewPage;
}

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
      throw new Error('Missing CSRF token cookie for test page setup');
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
    };

    const updateResponse = await fetch(`/api/pages/${createdPage.id}`, {
      method: 'PUT',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': csrfToken,
      },
      body: JSON.stringify({
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

async function expectEditAndSaveShortcutWorks(
  page: import('@playwright/test').Page,
  shortcutKeys: { editKey: string; saveKey: string },
) {
  const title = `Layout Shortcut Page ${Date.now()}`;
  const newContent = `Saved through layout-independent shortcut at ${new Date().toISOString()}`;

  await createPageAndOpenViewer(page, title);

  await dispatchLayoutShortcut(page, {
    key: shortcutKeys.editKey,
    code: 'KeyE',
    ctrlKey: true,
  });
  await page.locator('.cm-editor').waitFor({ state: 'visible' });

  const editPage = new EditPage(page);
  await editPage.writeContent(newContent);

  await dispatchLayoutShortcut(page, {
    key: shortcutKeys.saveKey,
    code: 'KeyS',
    ctrlKey: true,
  });
  await page.getByText('Page saved successfully').waitFor({ state: 'visible' });

  await editPage.closeEditor();

  await page.locator('article').getByText(newContent).waitFor({ state: 'visible' });
}

async function expectEditorFormattingShortcutsWork(
  page: import('@playwright/test').Page,
  shortcutKeys: { boldKey: string; italicKey: string },
) {
  const title = `Editor Shortcut Page ${Date.now()}`;
  const viewPage = await createPageAndOpenViewer(page, title);

  await viewPage.clickEditPageButton();

  const editPage = new EditPage(page);
  await editPage.writeContent('Intro\n');

  await dispatchLayoutShortcut(page, {
    key: shortcutKeys.boldKey,
    code: 'KeyB',
    ctrlKey: true,
  });
  await page.keyboard.type('Bold Text');
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');

  await editPage.writeContent('\n');

  await dispatchLayoutShortcut(page, {
    key: shortcutKeys.italicKey,
    code: 'KeyI',
    ctrlKey: true,
  });
  await page.keyboard.type('Italic Text');
  await page.keyboard.press('ArrowRight');

  await editPage.writeContent('\nHeading Line');

  await dispatchLayoutShortcut(page, {
    key: '1',
    code: 'Digit1',
    ctrlKey: true,
    altKey: true,
  });

  await editPage.savePage();
  await editPage.closeEditor();

  await page.locator('article strong').getByText('Bold Text').waitFor({
    state: 'visible',
  });
  await page.locator('article em').getByText('Italic Text').waitFor({
    state: 'visible',
  });
  await page
    .locator('article h1, article h2, article h3')
    .getByText('Heading Line')
    .waitFor({ state: 'visible' });
}

async function expectMarkdownLinkAutocompleteWorks(page: import('@playwright/test').Page) {
  const title = `Markdown Link Shortcut Page ${Date.now()}`;
  const viewPage = await createPageAndOpenViewer(page, title);

  await viewPage.clickEditPageButton();

  const editPage = new EditPage(page);
  await editPage.writeContent('[Welcome](/wel');

  const completionList = page.locator('.cm-tooltip-autocomplete');
  await completionList.waitFor({ state: 'visible' });
  const completionOption = completionList
    .locator('li')
    .filter({ hasText: 'Welcome to LeafWiki' })
    .first();
  await completionOption.waitFor({ state: 'visible' });
  await completionOption.click();
  await page.keyboard.type(')');

  await editPage.savePage();
  await editPage.closeEditor();

  const welcomeLink = page.locator(`article a[href="${toAppPath('/welcome-to-leafwiki')}"]`);
  await welcomeLink.getByText('Welcome').waitFor({ state: 'visible' });
}

async function expectOpenedPageMarkedInNavigationDuringEditMode(
  page: import('@playwright/test').Page,
) {
  const title = 'Welcome to LeafWiki';
  const viewPage = new ViewPage(page);
  await viewPage.goto('/welcome-to-leafwiki');

  const treeView = new TreeView(page);
  await treeView.expectPageHighlighted(title);

  await viewPage.clickEditPageButton();

  await treeView.expectPageHighlighted(title);

  const editPage = new EditPage(page);
  await editPage.closeEditor();
  await page.locator('article').waitFor({ state: 'visible' });
}

test.describe('Authenticated', () => {
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

  test('create-page', async ({ page }) => {
    const title = `My New Page ${Date.now()}`;

    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(title);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);
  });

  test('create-subpage', async ({ page }) => {
    const parentTitle = `Parent Page ${Date.now()}`;

    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(parentTitle);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);
    await treeView.createSubPageOfParent(parentTitle, `Child Page of ${parentTitle}`);
    await treeView.expectNumberOfTreeNodes(curNodeCount + 2);
  });

  test('sort-pages', async ({ page }) => {
    const parentTitle = `Sort Parent Page ${Date.now()}`;
    const childPages = ['Banana', 'Apple', 'Cherry', 'Date'];
    const desiredOrder = ['Apple', 'Banana', 'Cherry', 'Date'];

    // Create parent page
    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(parentTitle);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);

    // Create child pages
    await treeView.createMultipleSubPagesOfParent(parentTitle, childPages);
    await treeView.expectNumberOfTreeNodes(curNodeCount + childPages.length + 1);

    // Sort child pages
    await treeView.sortPagesOfParent(parentTitle, desiredOrder);
  });

  test('copy-markdown-code-block', async ({ page }) => {
    const title = `Copy Code Block ${Date.now()}`;
    const viewPage = await createPageAndOpenViewer(page, title);

    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.writeContent('```ts\nconst answer = 42;\nconsole.log(answer);\n```');
    await editPage.savePage();
    await editPage.closeEditor();

    const copyButton = page.locator('button[data-testid="markdown-code-copy-button"]').first();
    await copyButton.waitFor({ state: 'visible' });
    await copyButton.click();

    await page.getByText('Code copied').waitFor({ state: 'visible' });
  });

  test('view-page', async ({ page }) => {
    const title = `Page To View ${Date.now()}`;

    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(title);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);
    await treeView.clickPageByTitle(title);

    const viewPage = new ViewPage(page);
    const pageTitle = await viewPage.getTitle();
    test.expect(pageTitle).toBe(title);
  });

  test('edit-page', async ({ page }) => {
    const title = `Page To Edit ${Date.now()}`;
    const newContent = `This is the new content!  
**Bold Text**  

for the page edited at ${new Date().toISOString()}
`;

    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(title);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);
    await treeView.clickPageByTitle(title);

    const viewPage = new ViewPage(page);
    const pageTitle = await viewPage.getTitle();
    test.expect(pageTitle).toBe(title);

    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.writeContent(newContent);
    await editPage.savePage();
    await editPage.closeEditor();

    const content = await viewPage.getContent();
    test.expect(content).toContain('This is the new content!');
    test.expect(content).toContain('Bold Text');
  });

  test('opened page stays marked in navigation during edit mode without base path', async ({
    page,
  }) => {
    test.skip(e2eBasePath !== '', `Expected no base path, got "${e2eBasePath}"`);

    await expectOpenedPageMarkedInNavigationDuringEditMode(page);
  });

  test('opened page stays marked in navigation during edit mode with base path', async ({
    page,
  }) => {
    test.skip(e2eBasePath === '', 'Expected a configured base path for this test run');

    await expectOpenedPageMarkedInNavigationDuringEditMode(page);
  });

  test('layout-independent shortcuts work with latin keys', async ({ page }) => {
    await expectEditAndSaveShortcutWorks(page, {
      editKey: 'e',
      saveKey: 's',
    });
  });

  test('layout-independent shortcuts work with cyrillic keys', async ({ page }) => {
    await expectEditAndSaveShortcutWorks(page, {
      editKey: 'е',
      saveKey: 'с',
    });
  });

  test('editor formatting shortcuts work with latin keys', async ({ page }) => {
    await expectEditorFormattingShortcutsWork(page, {
      boldKey: 'b',
      italicKey: 'i',
    });
  });

  test('editor formatting shortcuts work with cyrillic keys', async ({ page }) => {
    await expectEditorFormattingShortcutsWork(page, {
      boldKey: 'б',
      italicKey: 'и',
    });
  });

  test('markdown link autocomplete works', async ({ page }) => {
    await expectMarkdownLinkAutocompleteWorks(page);
  });

  test('headline anchor keeps classic hash navigation for plain headings', async ({ page }) => {
    const timestamp = Date.now();
    const slug = `headline-anchor-${timestamp}`;
    const title = `Headline Anchor ${timestamp}`;
    const content = `# Intro

${Array.from({ length: 18 }, (_, index) => `Line ${index + 1}`).join('\n\n')}

## Anchor Target

Target content`;

    await createPageWithContent(page, { title, slug, content });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);

    const anchorTarget = page.locator('article h1').getByText('Intro');
    await anchorTarget.waitFor({ state: 'visible' });
    await anchorTarget.click();

    await test.expect
      .poll(async () => page.evaluate(() => window.location.hash), {
        timeout: 5000,
      })
      .toBe('#leafwiki-intro');
  });

  test('headline anchor supports non-ascii headings', async ({ page }) => {
    const timestamp = Date.now();
    const slug = `headline-anchor-unicode-${timestamp}`;
    const title = `Headline Anchor Unicode ${timestamp}`;
    const content = `# Привет мир

## Café Überblick

### 你好 世界`;

    await createPageWithContent(page, { title, slug, content });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);

    const cyrillicHeading = page.locator('article h1').getByText('Привет мир');
    await cyrillicHeading.waitFor({ state: 'visible' });
    await cyrillicHeading.click();

    await test.expect
      .poll(async () => page.evaluate(() => decodeURIComponent(window.location.hash)), {
        timeout: 5000,
      })
      .toBe('#leafwiki-привет-мир');

    const latinHeading = page.locator('article h2').getByText('Café Überblick');
    await latinHeading.waitFor({ state: 'visible' });
    await latinHeading.click();

    await test.expect
      .poll(async () => page.evaluate(() => decodeURIComponent(window.location.hash)), {
        timeout: 5000,
      })
      .toBe('#leafwiki-cafe-uberblick');

    const hanHeading = page.locator('article h3').getByText('你好 世界');
    await hanHeading.waitFor({ state: 'visible' });
    await hanHeading.click();

    await test.expect
      .poll(async () => page.evaluate(() => decodeURIComponent(window.location.hash)), {
        timeout: 5000,
      })
      .toBe('#leafwiki-你好-世界');
  });

  test('navigating away from page with footnote headline stays responsive', async ({ page }) => {
    const timestamp = Date.now();
    const slug = `footnotes-navigation-repro-${timestamp}`;
    const title = `Footnotes Navigation Repro ${timestamp}`;
    const content = `# Repro

This paragraph creates a footnote reference.[^leafwiki]

### [Footnotes](https://github.com/markdown-it/markdown-it-footnote)

[^leafwiki]: This is the matching footnote definition.`;

    await createPageWithContent(page, { title, slug, content });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);

    const contentText = await viewPage.getContent();
    test.expect(contentText).toContain('This paragraph creates a footnote reference.');
    test.expect(contentText).toContain('This is the matching footnote definition.');
    test
      .expect(
        await page
          .locator('article a[href="https://github.com/markdown-it/markdown-it-footnote"]')
          .count(),
      )
      .toBeGreaterThan(0);

    const reactErrors: string[] = [];
    page.on('console', (message) => {
      if (message.type() !== 'error') return;
      const text = message.text();
      if (
        /minified react error|cannot update a component while rendering a different component|maximum update depth exceeded/i.test(
          text,
        )
      ) {
        reactErrors.push(text);
      }
    });

    const pageErrors: string[] = [];
    page.on('pageerror', (error) => {
      pageErrors.push(error.message);
    });

    const treeView = new TreeView(page);
    await treeView.clickPageByTitle('Welcome to LeafWiki');

    await page.locator('article > h1').getByText('Welcome to LeafWiki').waitFor({
      state: 'visible',
      timeout: 10000,
    });

    test.expect(reactErrors).toEqual([]);
    test.expect(pageErrors).toEqual([]);
  });

  test('navigating away from markdown-it sample stays responsive', async ({ page }) => {
    const timestamp = Date.now();
    const slug = `markdown-it-sample-${timestamp}`;
    const title = `Markdown It Sample ${timestamp}`;
    const content = readFileSync(markdownItSamplePath, 'utf8');

    await createPageWithContent(page, { title, slug, content });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);

    const contentText = await viewPage.getContent();
    test.expect(contentText).toContain('h1 Heading 8-)');
    test.expect(contentText).toContain('Footnote text.');
    test.expect(contentText).toContain('This is HTML abbreviation example.');

    const reactErrors: string[] = [];
    page.on('console', (message) => {
      if (message.type() !== 'error') return;
      const text = message.text();
      if (
        /minified react error|cannot update a component while rendering a different component|maximum update depth exceeded/i.test(
          text,
        )
      ) {
        reactErrors.push(text);
      }
    });

    const pageErrors: string[] = [];
    page.on('pageerror', (error) => {
      pageErrors.push(error.message);
    });

    const treeView = new TreeView(page);
    await treeView.clickPageByTitle('Welcome to LeafWiki');

    await page.locator('article > h1').getByText('Welcome to LeafWiki').waitFor({
      state: 'visible',
      timeout: 10000,
    });

    test.expect(reactErrors).toEqual([]);
    test.expect(pageErrors).toEqual([]);
  });

  test('unsaved changes-warning', async ({ page }) => {
    const title = `Page With Unsaved Changes ${Date.now()}`;
    const newContent = `This is some unsaved content!  
**Unsaved Bold Text**  

for the page edited at ${new Date().toISOString()}
`;

    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(title);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);
    await treeView.clickPageByTitle(title);

    const viewPage = new ViewPage(page);
    const pageTitle = await viewPage.getTitle();
    test.expect(pageTitle).toBe(title);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.writeContent(newContent);

    let dialogType: string | undefined;

    page.once('dialog', (dialog) => {
      dialogType = dialog.type();
      dialog.dismiss().catch(() => {
        // Ignore errors from dismissing the dialog
      });
    });

    let navError: unknown = null;

    try {
      await page.goto(toAppPath('/'));
    } catch (e) {
      navError = e;
    }

    test.expect(dialogType).toBe('beforeunload');

    test.expect(String((navError as Error)?.message ?? '')).toMatch(/ERR_ABORTED/);
  });

  test('create-page-with-mermaid', async ({ page }) => {
    const title = `Page With Mermaid ${Date.now()}`;
    const mermaidContent = `\`\`\`mermaid
graph TD;
    A-->B;
\`\`\``;

    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(title);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);
    await treeView.clickPageByTitle(title);

    const viewPage = new ViewPage(page);
    const pageTitle = await viewPage.getTitle();
    test.expect(pageTitle).toBe(title);

    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.writeContent(mermaidContent);
    await editPage.savePage();
    await editPage.closeEditor();

    // expects at least one SVG element (the mermaid diagram)
    const svgCount = await viewPage.amountOfSVGElements();
    test.expect(svgCount).toBeGreaterThan(0);
  });

  test('invalid mermaid degrades locally without page crash', async ({ page }) => {
    const timestamp = Date.now();
    const slug = `invalid-mermaid-${timestamp}`;
    const title = `Invalid Mermaid ${timestamp}`;
    const content = `# Invalid Mermaid

\`\`\`mermaid
graph TD
A -->
\`\`\``;

    await createPageWithContent(page, { title, slug, content });

    const pageErrors: string[] = [];
    page.on('pageerror', (error) => {
      pageErrors.push(error.message);
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);

    await page.getByText('Unable to render Mermaid diagram.').waitFor({
      state: 'visible',
      timeout: 10000,
    });
    await page.locator('article pre code').getByText('graph TD').waitFor({
      state: 'visible',
    });

    const treeView = new TreeView(page);
    await treeView.clickPageByTitle('Welcome to LeafWiki');
    await page.locator('article > h1').getByText('Welcome to LeafWiki').waitFor({
      state: 'visible',
      timeout: 10000,
    });

    test.expect(pageErrors).toEqual([]);
  });

  test('light-mode preview uses light syntax highlighting and mermaid theme', async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('design-mode', 'light');
    });

    const title = `Light Mode Preview ${Date.now()}`;
    const content = `Inline \`const foo = 1\`

\`\`\`ts
const greeting = 'hello';
function sum(a: number, b: number) {
  return a + b;
}
\`\`\`

\`\`\`mermaid
graph TD;
    Light-->Preview;
\`\`\``;

    const viewPage = await createPageAndOpenViewer(page, title);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.writeContent(content);
    await editPage.savePage();
    await editPage.closeEditor();

    const inlineCode = page.locator('article code.inline-code').first();
    await inlineCode.waitFor({ state: 'visible' });

    const codeBlock = page.locator('article pre code.hljs').first();
    await codeBlock.waitFor({ state: 'visible' });

    const codeBlockContainer = page
      .locator('article pre')
      .filter({ has: page.locator('code.hljs') })
      .first();
    await codeBlockContainer.waitFor({ state: 'visible' });

    const mermaidSvg = page.locator('article .my-4 svg').first();
    await mermaidSvg.waitFor({ state: 'visible' });

    const pageViewer = page.locator('.page-viewer__content').first();

    const inlineStyles = await inlineCode.evaluate((element) => {
      const styles = window.getComputedStyle(element);
      const parentStyles = window.getComputedStyle(element.parentElement as Element);
      return {
        backgroundColor: styles.backgroundColor,
        color: styles.color,
        parentColor: parentStyles.color,
      };
    });

    test.expect(inlineStyles.backgroundColor).not.toBe('rgba(0, 0, 0, 0)');
    test.expect(inlineStyles.color).toBe(inlineStyles.parentColor);

    const viewerBackground = await pageViewer.evaluate((element) => {
      return window.getComputedStyle(element).backgroundColor;
    });

    const codeBlockContainerStyles = await codeBlockContainer.evaluate((element) => {
      const styles = window.getComputedStyle(element);
      return {
        backgroundColor: styles.backgroundColor,
        color: styles.color,
        borderTopColor: styles.borderTopColor,
      };
    });

    test.expect(codeBlockContainerStyles.backgroundColor).not.toBe(viewerBackground);
    test.expect(codeBlockContainerStyles.borderTopColor).not.toBe('rgba(0, 0, 0, 0)');

    const codeBlockStyles = await codeBlock.evaluate((element) => {
      const styles = window.getComputedStyle(element);
      const keyword = element.querySelector('.hljs-keyword');
      const keywordStyles = keyword ? window.getComputedStyle(keyword) : null;

      return {
        backgroundColor: styles.backgroundColor,
        color: styles.color,
        keywordColor: keywordStyles?.color ?? null,
      };
    });

    test.expect(codeBlockStyles.backgroundColor).toBe(codeBlockContainerStyles.backgroundColor);
    test.expect(codeBlockStyles.keywordColor).not.toBe(codeBlockStyles.color);

    const mermaidContainerStyles = await mermaidSvg.evaluate((element) => {
      const container = element.closest('pre');
      if (!container) {
        throw new Error('Mermaid pre container not found');
      }

      const styles = window.getComputedStyle(container);
      return {
        backgroundColor: styles.backgroundColor,
        color: styles.color,
        borderTopColor: styles.borderTopColor,
      };
    });

    test
      .expect(mermaidContainerStyles.backgroundColor)
      .toBe(codeBlockContainerStyles.backgroundColor);
    test
      .expect(mermaidContainerStyles.borderTopColor)
      .toBe(codeBlockContainerStyles.borderTopColor);

    const mermaidStyles = await mermaidSvg.evaluate((element) => {
      const styles = window.getComputedStyle(element);
      const firstNode = element.querySelector(
        '.node rect, .node polygon, .node circle, .node ellipse',
      ) as SVGGraphicsElement | null;
      const nodeStyles = firstNode ? window.getComputedStyle(firstNode) : null;

      return {
        backgroundColor: styles.backgroundColor,
        fill: nodeStyles?.fill ?? null,
        stroke: nodeStyles?.stroke ?? null,
      };
    });

    test.expect(mermaidStyles.fill).not.toBe('rgb(30, 30, 30)');
    test.expect(mermaidStyles.stroke).not.toBe('rgb(231, 231, 231)');
  });

  test('create-page-on-not-found-page', async ({ page }) => {
    const slug = `page-from-not-found-${Date.now()}`;
    const pagePath = `/${slug}`;

    const notfoundPage = new NotFoundPage(page);
    await notfoundPage.goto(pagePath);

    test.expect(await notfoundPage.isNotFoundPage()).toBeTruthy();

    await notfoundPage.clickCreatePageButton();
    const createPageByPathDialog = new CreatePageByPathDialog(page);
    await createPageByPathDialog.clickCreate();

    // Check if we are in edit mode
    const editPage = new EditPage(page);
    await editPage.closeEditor();

    // Verify page creation
    const viewPage = new ViewPage(page);
    const pageTitle = await viewPage.getTitle();
    test.expect(pageTitle).toBe(slug);
  });

  // test move
  test('move-page-subpage-to-root-level', async ({ page }) => {
    const parentTitle = `Move Parent Page ${Date.now()}`;
    const childTitle = `Child Page of ${parentTitle}`;

    // Create parent page
    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(parentTitle);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);
    // Create child page
    await treeView.createSubPageOfParent(parentTitle, childTitle);
    await treeView.expectNumberOfTreeNodes(curNodeCount + 2);

    // Move child page to root level
    await treeView.movePageToTopLevel(parentTitle, childTitle);
    // Verify the move
    const nodeRow = page
      .locator('div[data-testid^="tree-node-"]')
      .filter({ hasText: childTitle })
      .first();

    test.expect(await nodeRow.count()).toBe(1);
  });

  test('copy-page', async ({ page }) => {
    const title = `Page To Copy ${Date.now()}`;

    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(title);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);
    await treeView.clickPageByTitle(title);

    const viewPage = new ViewPage(page);
    const pageTitle = await viewPage.getTitle();
    test.expect(pageTitle).toBe(title);

    const copyPageDialog = new CopyPageDialog(page);
    await viewPage.clickCopyPageButton();

    const newTitle = `Copy of ${title}`;
    await copyPageDialog.fillTitle(newTitle);
    await copyPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 2);
  });

  test('delete-page', async ({ page }) => {
    const title = `Page To Delete ${Date.now()}`;

    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(title);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);
    await treeView.clickPageByTitle(title);

    const viewPage = new ViewPage(page);
    const pageTitle = await viewPage.getTitle();
    test.expect(pageTitle).toBe(title);

    await viewPage.clickDeletePageButton();

    const deletePageDialog = new DeletePageDialog(page);
    test.expect(await deletePageDialog.dialogTextVisible()).toBeTruthy();
    await deletePageDialog.abortDeletion();
    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);

    await viewPage.clickDeletePageButton();
    test.expect(await deletePageDialog.dialogTextVisible()).toBeTruthy();
    await deletePageDialog.confirmDeletion();
    await treeView.expectNumberOfTreeNodes(curNodeCount);
  });

  test('nested-delete-operation', async ({ page }) => {
    const parentTitle = `Delete Parent Page ${Date.now()}`;
    const childTitle = `Child Page ${Date.now()}`;

    // Create parent page
    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(parentTitle);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);
    // Create child page
    await treeView.createSubPageOfParent(parentTitle, childTitle);
    await treeView.expectNumberOfTreeNodes(curNodeCount + 2);

    // Delete parent page
    await treeView.clickPageByTitle(parentTitle);
    const viewPage = new ViewPage(page);
    await viewPage.clickDeletePageButton();

    const deletePageDialog = new DeletePageDialog(page);
    test.expect(await deletePageDialog.dialogTextVisible()).toBeTruthy();
    await deletePageDialog.confirmDeletion();

    // The dialog stays open, because we need to confirm nested deletion
    test.expect(await deletePageDialog.dialogTextVisible()).toBeTruthy();

    await deletePageDialog.confirmNestedDeletion();
    await treeView.expectNumberOfTreeNodes(curNodeCount);
    // Dialog should be closed now
    test.expect(await deletePageDialog.dialogTextVisible()).toBeFalsy();
  });

  // disable this test cases, because it is flaky
  // TODO: fix the flakiness
  /*
  test('search-page', async ({ page }) => {
    const title = `Page To Search ${Date.now()}`;
    const content = `This is the content of the page to search, created at ${new Date().toISOString()}`;

    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(title);
    await addPageDialog.submitWithoutRedirect();

    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);

    // open edit mode
    await treeView.clickPageByTitle(title);
    const viewPage = new ViewPage(page);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.writeContent(content);
    await editPage.savePage();
    await editPage.closeEditor();

    // switch to search tab
    await viewPage.switchToSearchTab();

    const searchView = new SearchView(page);
    await searchView.enterSearchQuery(title);

    const result = await searchView.searchResultContainsPageTitle(title);
    test.expect(result).toBeTruthy();

    // clear search
    await searchView.clearSearch();
  });
  */

  test('markdown-relative-link-navigates-to-sibling-page', async ({ page }) => {
    const suffix = Date.now();
    const parentTitle = 'markdown-link-parent-' + suffix;
    const sourceTitle = 'source-' + suffix;
    const targetTitle = 'target-' + suffix;
    const linkLabel = 'Go to sibling target';

    const treeView = new TreeView(page);
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(parentTitle);
    await addPageDialog.submitWithoutRedirect();

    await treeView.createSubPageOfParent(parentTitle, sourceTitle);
    await treeView.createSubPageOfParent(parentTitle, targetTitle);
    await treeView.expandNodeByTitle(parentTitle);
    await treeView.clickPageByTitle(sourceTitle);

    const viewPage = new ViewPage(page);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.writeContent('[' + linkLabel + '](../' + targetTitle + ')');
    await editPage.savePage();
    await editPage.closeEditor();

    await page.getByRole('link', { name: linkLabel }).click();
    await page.waitForURL(new RegExp('/' + parentTitle + '/' + targetTitle + '$'));

    await test.expect(page.locator('article>h1')).toHaveText(targetTitle);
  });

  test('delete-current-subpage-redirects-to-parent-page', async ({ page }) => {
    const suffix = Date.now();
    const parentTitle = 'delete-parent-' + suffix;
    const childTitle = 'delete-child-' + suffix;

    const treeView = new TreeView(page);
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(parentTitle);
    await addPageDialog.submitWithoutRedirect();

    await treeView.createSubPageOfParent(parentTitle, childTitle);
    await treeView.expandNodeByTitle(parentTitle);
    await treeView.clickPageByTitle(childTitle);

    const viewPage = new ViewPage(page);
    await viewPage.clickDeletePageButton();

    const deletePageDialog = new DeletePageDialog(page);
    await deletePageDialog.confirmDeletion();
    await page.waitForURL(new RegExp('/' + parentTitle + '$'));

    test.expect(await viewPage.getTitle()).toBe(parentTitle);
  });

  test('delete-unrelated-page-keeps-current-page-open', async ({ page }) => {
    const suffix = Date.now();
    const currentTitle = 'current-page-' + suffix;
    const otherTitle = 'other-page-' + suffix;

    const treeView = new TreeView(page);
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(currentTitle);
    await addPageDialog.submitWithoutRedirect();

    await treeView.clickRootAddButton();
    await addPageDialog.fillTitle(otherTitle);
    await addPageDialog.submitWithoutRedirect();

    await treeView.clickPageByTitle(currentTitle);

    const viewPage = new ViewPage(page);
    test.expect(await viewPage.getTitle()).toBe(currentTitle);

    const nodeRow = page
      .locator('div[data-testid^="tree-node-"]')
      .filter({ hasText: otherTitle })
      .first();

    await nodeRow.scrollIntoViewIfNeeded();
    await nodeRow.hover();

    const moreActionsButton = nodeRow.locator(
      'button[data-testid="tree-view-action-button-open-more-actions"]',
    );
    await moreActionsButton.click({ force: true });

    const deleteButton = page.locator('div[data-testid="tree-view-action-button-delete"]');
    await deleteButton.click({ force: true });

    const deletePageDialog = new DeletePageDialog(page);
    await deletePageDialog.confirmDeletion();

    test.expect(await viewPage.getTitle()).toBe(currentTitle);
    await page.waitForURL(new RegExp('/' + currentTitle + '$'));
  });

  test('cannot-delete-current-page-while-editing-it', async ({ page }) => {
    const title = 'Editing Delete Guard ' + Date.now();
    const warningText =
      'This page is currently being edited. Please close the editor before deleting it.';

    const treeView = new TreeView(page);
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(title);
    await addPageDialog.submitWithoutRedirect();

    await treeView.clickPageByTitle(title);

    const viewPage = new ViewPage(page);
    await viewPage.clickEditPageButton();

    const nodeRow = page
      .locator('div[data-testid^="tree-node-"]')
      .filter({ hasText: title })
      .first();

    await nodeRow.scrollIntoViewIfNeeded();
    await nodeRow.hover();

    const moreActionsButton = nodeRow.locator(
      'button[data-testid="tree-view-action-button-open-more-actions"]',
    );
    await moreActionsButton.click({ force: true });

    const deleteButton = page.locator('div[data-testid="tree-view-action-button-delete"]');
    await deleteButton.click({ force: true });

    const deletePageDialog = new DeletePageDialog(page);
    test.expect(await deletePageDialog.dialogTextVisible()).toBeFalsy();
    await page.getByText(warningText).waitFor({ state: 'visible' });
    test.expect(await page.locator('.cm-editor').isVisible()).toBeTruthy();
  });

  test('edit-metadata-on-nested-page-keeps-parent-path', async ({ page }) => {
    const suffix = Date.now();
    const parentTitle = 'meta-parent-' + suffix;
    const childTitle = 'meta-child-' + suffix;
    const renamedChildTitle = 'meta-child-renamed-' + suffix;
    const expectedPath = parentTitle + '/' + renamedChildTitle;

    const treeView = new TreeView(page);
    await treeView.clickRootAddButton();

    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(parentTitle);
    await addPageDialog.submitWithoutRedirect();

    await treeView.createSubPageOfParent(parentTitle, childTitle);
    await treeView.expandNodeByTitle(parentTitle);
    await treeView.clickPageByTitle(childTitle);

    const viewPage = new ViewPage(page);
    await viewPage.clickEditPageButton();

    const editPage = new EditPage(page);
    await editPage.openMetadataDialog();

    const metadataDialog = new EditPageMetadataDialog(page);
    await metadataDialog.fillTitle(renamedChildTitle);
    await metadataDialog.expectSlug(renamedChildTitle);
    await metadataDialog.expectPath(expectedPath);
    await metadataDialog.submit();

    await editPage.savePage();
    await editPage.closeEditor();

    await page.waitForURL(new RegExp('/' + expectedPath + '$'));
  });

  test('test-asset-upload-and-use-in-page', async ({ page }) => {
    const title = `Page With Asset ${Date.now()}`;
    // const assetFileName = 'test-image.png';
    const treeView = new TreeView(page);
    const curNodeCount = await treeView.getNumberOfTreeNodes();
    await treeView.clickRootAddButton();
    const addPageDialog = new AddPageDialog(page);
    await addPageDialog.fillTitle(title);
    await addPageDialog.submitWithoutRedirect();
    await treeView.expectNumberOfTreeNodes(curNodeCount + 1);
    await treeView.clickPageByTitle(title);
    let viewPage = new ViewPage(page);
    const pageTitle = await viewPage.getTitle();
    test.expect(pageTitle).toBe(title);
    await viewPage.clickEditPageButton();
    // pause to see the editor
    const editPage = new EditPage(page);
    // Opens asset manager in edit mode
    editPage.openAssetManager();
    // Upload asset
    await editPage.uploadAsset(currentDir + '/../assets/upload-test.png');
    await editPage.listAmountOfAssets().then((count) => {
      test.expect(count).toBeGreaterThan(0);
    });
    // Insert first asset into page
    await editPage.insertFirstAssetIntoPage();
    await editPage.savePage();
    await editPage.closeEditor();
    viewPage = new ViewPage(page);
    await viewPage.amountOfImages().then((count) => {
      test.expect(count).toBeGreaterThan(0);
    });
  });
});
