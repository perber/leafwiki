import { Page } from '@playwright/test';

export default class SearchView {
  constructor(private page: Page) {}

  async getSearchInput() {
    return this.page.locator('input[data-testid="search-input"]');
  }

  async enterSearchQuery(query: string) {
    const searchInput = await this.getSearchInput();
    await searchInput.fill(query);
  }

  async clearSearch() {
    const clearButton = this.page.locator('button[data-testid="search-clear-button"]');
    await clearButton.click();
    // wait for results to update
    await this.page.waitForTimeout(500);
  }

  async searchResultContainsPageTitle(title: string): Promise<boolean> {
    // find all results
    // check if any contains the title
    // wait for a[data-testid^="search-result-card-"] to be visible
    await this.page.waitForSelector('a[data-testid^="search-result-card-"]', { state: 'visible' });
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
}
