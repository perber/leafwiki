import { expect, test } from '@playwright/test';
import { createPageViaApi, loggedIn } from '../helpers/apiContext';
import EditPage from '../pages/EditPage';
import LoginPage from '../pages/LoginPage';
import SearchView from '../pages/SearchView';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

async function createPage(slug: string, title: string, content: string) {
  const api = await loggedIn();
  await createPageViaApi(api, { title, slug, content });
  await api.dispose();
}

test.describe('Search: short queries and syntax help', () => {
  test.beforeEach(async ({ page }) => {
    await new LoginPage(page).loginViaApi(user, password);
    await new ViewPage(page).expectUserLoggedIn();
  });

  test('a two-character query finds a page, one character shows the hint', async ({ page }) => {
    const stamp = Date.now();
    const title = `Short Query ${stamp}`;
    await createPage(`short-query-${stamp}`, title, 'The value of Pi is roughly 3.14.');

    const viewPage = new ViewPage(page);
    await viewPage.goto('/');
    await viewPage.switchToSearchTab();
    const searchView = new SearchView(page);

    await searchView.enterSearchQuery('Pi');
    await expect(searchView.getResultCards().filter({ hasText: title })).toBeVisible();
    await expect(searchView.getMinLengthHint()).toHaveCount(0);

    await searchView.enterTooShortSearchQuery('P');
    await expect(
      page.getByRole('status').filter({ has: searchView.getMinLengthHint() }),
    ).toBeVisible();
    await expect(searchView.getResultCards()).toHaveCount(0);
    await expect(searchView.getResultsList()).toHaveCount(0);
    await expect(page.locator('.search__result-summary')).toHaveCount(0);

    // Whitespace does not count towards the minimum.
    await searchView.enterTooShortSearchQuery(' P ');
    await expect(searchView.getResultCards()).toHaveCount(0);

    // Back to two characters: results return, the hint goes away.
    await searchView.enterSearchQuery('Pi');
    await expect(searchView.getResultCards().filter({ hasText: title })).toBeVisible();
    await expect(searchView.getMinLengthHint()).toHaveCount(0);
  });

  test('the ? button toggles the syntax help, Escape closes it and refocuses ?', async ({
    page,
  }) => {
    const viewPage = new ViewPage(page);
    await viewPage.goto('/');
    await viewPage.switchToSearchTab();
    const searchView = new SearchView(page);
    const button = searchView.getSyntaxHelpButton();

    await expect(button).toHaveAttribute('aria-expanded', 'false');
    await expect(searchView.getSyntaxHelpPanel()).toBeHidden();

    await searchView.openSyntaxHelp();
    await expect(button).toHaveAttribute(
      'aria-controls',
      (await searchView.getSyntaxHelpPanel().getAttribute('id')) ?? '',
    );

    await page.keyboard.press('Escape');
    await expect(searchView.getSyntaxHelpPanel()).toBeHidden();
    await expect(button).toHaveAttribute('aria-expanded', 'false');
    await expect(button).toBeFocused();

    // Second click closes it again; typing keeps it open.
    await searchView.openSyntaxHelp();
    await searchView.enterSearchQuery('title:foo');
    await expect(searchView.getSyntaxHelpPanel()).toBeVisible();
    // Escape from the search input closes it too.
    await searchView.getSearchInput().focus();
    await page.keyboard.press('Escape');
    await expect(searchView.getSyntaxHelpPanel()).toBeHidden();
    await expect(button).toBeFocused();

    await searchView.openSyntaxHelp();
    await button.click();
    await expect(button).toHaveAttribute('aria-expanded', 'false');
    await expect(searchView.getSyntaxHelpPanel()).toBeHidden();
  });

  test('on the history page the first Escape closes the help, the second leaves history', async ({
    page,
  }) => {
    const stamp = Date.now();
    const slug = `short-query-history-${stamp}`;
    await createPage(slug, `Short Query History ${stamp}`, 'Some content for history.');

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.openCurrentPageHistory();
    await expect.poll(() => new URL(page.url()).pathname).toContain(`/history/${slug}`);

    await viewPage.switchToSearchTab();
    const searchView = new SearchView(page);
    await searchView.openSyntaxHelp();

    await page.keyboard.press('Escape');
    await expect(searchView.getSyntaxHelpPanel()).toBeHidden();
    await expect(searchView.getSyntaxHelpButton()).toBeFocused();
    await expect(page.getByTestId('page-history-page-content')).toBeVisible();
    expect(new URL(page.url()).pathname).toContain(`/history/${slug}`);

    await page.keyboard.press('Escape');
    await expect.poll(() => new URL(page.url()).pathname).not.toContain('/history/');
    await expect(page.locator('article')).toBeVisible();
  });

  test('in the editor, Escape closes the CodeMirror search panel and leaves the help open', async ({
    page,
  }) => {
    const stamp = Date.now();
    const slug = `short-query-editor-${stamp}`;
    await createPage(slug, `Short Query Editor ${stamp}`, 'Editor content.');

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await viewPage.clickEditPageButton();
    await viewPage.switchToSearchTab();
    const searchView = new SearchView(page);
    await searchView.openSyntaxHelp();

    const editPage = new EditPage(page);
    await editPage.openReplacePanel();
    await editPage.closeSearchPanelWithEscape();
    await editPage.expectEditorStillOpen();
    await expect(searchView.getSyntaxHelpPanel()).toBeVisible();
  });
});
