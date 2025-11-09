import { Page } from '@playwright/test';

export default class SortPageDialog {
    constructor(private page: Page) { }

    async getCurrentOrder(): Promise<string[]> {
        const items = this.page.locator('li[data-testid^="sort-page-item-"]');
        const count = await items.count();
        const titles: string[] = [];

        for (let i = 0; i < count; i++) {
            const title = await items
                .nth(i)
                .locator('span[data-testid^="sort-page-title-"]')
                .innerText();
            titles.push(title);
        }

        return titles;
    }

    async sortPageItems(plannedOrder: string[]) {
        const items = this.page.locator('li[data-testid^="sort-page-item-"]');
        const count = await items.count();

        const titles: string[] = [];
        for (let i = 0; i < count; i++) {
            const title = await items.nth(i)
                .locator('span[data-testid^="sort-page-title-"]')
                .innerText();
            titles.push(title);
        }

        for (let i = 0; i < plannedOrder.length; i++) {
            const desiredTitle = plannedOrder[i];
            const currentTitle = titles[i];

            if (currentTitle === desiredTitle) {
                continue;
            }

            let desiredIndex = titles.indexOf(desiredTitle);
            if (desiredIndex === -1) {
                throw new Error(`Title "${desiredTitle}" not found in current order`);
            }

            const pageId = await this.getPageIdByTitle(desiredTitle);

            while (desiredIndex > i) {
                const moveUpButton = this.page.locator(
                    `button[data-testid="move-up-button-${pageId}"]`
                );
                await moveUpButton.click();


                [titles[desiredIndex - 1], titles[desiredIndex]] =
                    [titles[desiredIndex], titles[desiredIndex - 1]];

                desiredIndex--;
            }


            while (desiredIndex < i) {
                const moveDownButton = this.page.locator(
                    `button[data-testid="move-down-button-${pageId}"]`
                );
                await moveDownButton.click();

                [titles[desiredIndex], titles[desiredIndex + 1]] =
                    [titles[desiredIndex + 1], titles[desiredIndex]];

                desiredIndex++;
            }
        }
    }

    async getPageIdByTitle(title: string): Promise<string> {
        const items = this.page.locator('li[data-testid^="sort-page-item-"]');
        const count = await items.count();
        for (let i = 0; i < count; i++) {
            const item = items.nth(i);
            const itemTitle = await item.locator('span[data-testid^="sort-page-title-"]').innerText();
            if (itemTitle === title) {
                const testId = await item.getAttribute('data-testid');
                if (testId) {
                    return testId.replace('sort-page-item-', '');
                }
            }
        }
        throw new Error(`Page with title "${title}" not found`);
    }

    async saveSorting() {
        const saveButton = this.page.locator('button[data-testid="sort-pages-dialog-save-button"]');
        await saveButton.click();
        // Wait a bit to ensure the sorting is processed
        await this.page.waitForTimeout(1000);
    }
}
