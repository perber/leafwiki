import { expect, Page } from '@playwright/test';

export default class TagsView {
  constructor(private page: Page) {}

  async open() {
    await this.page.locator('button[data-testid="sidebar-tags-tab-button"]').click();
    await this.page.locator('input[data-testid="tags-search-input"]').waitFor({ state: 'visible' });
  }

  getSearchInput() {
    return this.page.locator('input[data-testid="tags-search-input"]');
  }

  getSuggestion(tag: string) {
    return this.page.getByTestId(`tags-suggestion-${tag}`);
  }

  getSelectedChip(tag: string) {
    return this.page.getByTestId(`tags-selected-chip-${tag}`);
  }

  getResultsList() {
    return this.page.getByTestId('tags-results-list');
  }

  getResultTitle(text: string) {
    return this.page.locator('.browse-results__item-title').filter({ hasText: text });
  }

  getFetchError() {
    return this.page.getByTestId('tags-fetch-error');
  }

  getNoResults() {
    return this.page.locator('.browse-results__empty').filter({ hasText: 'No pages found' });
  }

  async typeTag(tag: string) {
    await this.getSearchInput().fill(tag);
  }

  async selectSuggestion(tag: string) {
    const suggestion = this.getSuggestion(tag);
    await suggestion.waitFor({ state: 'visible' });
    await suggestion.click();
  }

  async waitForResults() {
    await this.getResultsList().waitFor({ state: 'visible' });
  }

  async expectResultVisible(title: string) {
    await expect(this.getResultTitle(title)).toBeVisible();
  }

  async expectResultNotVisible(title: string) {
    await expect(this.getResultTitle(title)).toHaveCount(0);
  }

  async expectChipVisible(tag: string) {
    await expect(this.getSelectedChip(tag)).toBeVisible();
  }

  async clearFilter() {
    await this.page.locator('.browse-results__clear').click();
  }
}
