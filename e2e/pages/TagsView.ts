import { expect, Page } from '@playwright/test';

export default class TagsView {
  constructor(private page: Page) {}

  async open() {
    await this.page.getByTestId('sidebar-search-tab-button').click();
    await this.page.getByTestId('search-input').waitFor({ state: 'visible' });
  }

  getTagFilter(tag: string) {
    return this.page.getByTestId(`tags-filter-${tag}`);
  }

  getSelectedChip(tag: string) {
    return this.getTagFilter(tag);
  }

  getResultsList() {
    return this.page.getByTestId('search-results-list');
  }

  getResultTitle(text: string) {
    return this.page
      .locator('[data-testid^="search-result-card-title-"]')
      .filter({ hasText: text });
  }

  getNoResults() {
    return this.page.locator('.search__result-summary').filter({ hasText: 'No results found.' });
  }

  async clickTagFilter(tag: string) {
    await this.ensureTagListVisible();
    await this.getTagFilter(tag).waitFor({ state: 'visible' });
    await this.getTagFilter(tag).click();
  }

  async waitForResults() {
    await this.getResultsList().waitFor({ state: 'visible' });
  }

  async waitForEmptyState() {
    await this.getNoResults().waitFor({ state: 'visible' });
  }

  async expectResultVisible(title: string) {
    await expect(this.getResultTitle(title)).toBeVisible();
  }

  async expectResultNotVisible(title: string) {
    await expect(this.getResultTitle(title)).toHaveCount(0);
  }

  async expectChipVisible(tag: string) {
    await expect(this.page).toHaveURL(new RegExp(`tags=${tag}`));
  }

  async expectChipNotVisible(tag: string) {
    await expect(this.page).not.toHaveURL(new RegExp(`tags=${tag}`));
  }

  async clearFilter() {
    await this.page.getByTestId('search-tags-clear-button').click();
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
