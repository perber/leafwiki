import { expect, Page } from '@playwright/test';

export default class SearchView {
  constructor(private page: Page) {}

  async getSearchInput() {
    return this.page.locator('input[data-testid="search-input"]');
  }

  async enterSearchQuery(query: string) {
    const searchInput = await this.getSearchInput();
    await searchInput.fill(query);
    await this.page
      .locator('.search__result-summary, a[data-testid^="search-result-card-"]')
      .first()
      .waitFor({ state: 'visible' });
  }

  async clearSearch() {
    const clearButton = this.page.locator('button[data-testid="search-clear-button"]');
    await clearButton.click();
    await expect(await this.getSearchInput()).toHaveValue('');
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

  async expectResultHighlighted(title: string) {
    const result = this.page
      .locator('a[data-testid^="search-result-card-"]')
      .filter({ hasText: title })
      .first();

    await result.waitFor({ state: 'visible' });
    await expect(result).toHaveAttribute('aria-current', 'page');
  }
}
