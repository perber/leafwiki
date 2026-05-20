import { expect, Page } from '@playwright/test';

export default class SearchView {
  constructor(private page: Page) {}

  getSearchInput() {
    return this.page.locator('input[data-testid="search-input"]');
  }

  getResultsList() {
    return this.page.getByTestId('search-results-list');
  }

  getTagAccordion() {
    return this.page.getByTestId('search-tags-accordion');
  }

  getTagFilter(tag: string) {
    return this.page.getByTestId(`tags-filter-${tag}`);
  }

  getTagFilters() {
    return this.page.locator('[data-testid^="tags-filter-"]');
  }

  async enterSearchQuery(query: string) {
    const searchInput = this.getSearchInput();
    await searchInput.fill(query);
    await this.page
      .locator('.search__result-summary, [data-testid="search-results-list"]')
      .first()
      .waitFor({ state: 'visible' });
  }

  async clearSearch() {
    const clearButton = this.page.locator('button[data-testid="search-clear-button"]');
    await clearButton.click();
    await expect(this.getSearchInput()).toHaveValue('');
  }

  async clickTagFilter(tag: string) {
    await this.ensureTagListVisible();
    await this.getTagFilter(tag).click();
    await this.page
      .locator('.search__result-summary, [data-testid="search-results-list"]')
      .first()
      .waitFor({ state: 'visible' });
  }

  async searchResultContainsPageTitle(title: string): Promise<boolean> {
    // find all results
    // check if any contains the title
    // wait for a[data-testid^="search-result-card-"] to be visible
    await this.page.locator('a[data-testid^="search-result-card-"]').waitFor({
      state: 'visible',
    });
    const results = this.page.locator('a[data-testid^="search-result-card-"]');
    const count = await results.count();
    for (let i = 0; i < count; i++) {
      const result = results.nth(i);
      const text = await result.locator(`[data-testid^="search-result-card-title-"]`).innerText();
      if (text.includes(title)) {
        return true;
      }
    }
    return false;
  }

  async expectSearchResultMissing(title: string) {
    await expect(
      this.page.locator('a[data-testid^="search-result-card-"]').filter({ hasText: title }),
    ).toHaveCount(0);
  }

  async expectResultHighlighted(title: string) {
    const result = this.page
      .locator('a[data-testid^="search-result-card-"]')
      .filter({ hasText: title })
      .first();

    await result.waitFor({ state: 'visible' });
    await expect(result).toHaveAttribute('aria-current', 'page');
  }

  private async ensureTagListVisible() {
    const tagList = this.page.getByTestId('search-tags-list');
    if (await tagList.isVisible().catch(() => false)) {
      return;
    }

    await this.page.getByTestId('search-tags-accordion-trigger').click();
    await tagList.waitFor({ state: 'visible' });
  }
}
