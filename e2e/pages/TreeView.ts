import { expect, Page } from '@playwright/test';

export default class TreeView {
    constructor(private page: Page) { }

    async getRootAddButton() {
        return this.page.locator('button[data-testid="tree-view-action-button-add"]');
    }

    async clickRootAddButton() {
        if (!(await this.isSidebarVisible())) {
            await this.page.getByTestId('sidebar-toggle-button').click();
        }
        await (await this.getRootAddButton()).click();
    }

    async isSidebarVisible(): Promise<boolean> {
        return this.page.getByTestId('sidebar').isVisible();
    }

    async getNumberOfTreeNodes() {
        return this.page.locator('a[data-testid^="tree-node-link-"]').count();
    }

    async findPageByTitle(title: string) {
        return this.page.locator(`a[data-testid^="tree-node-link-"] >> text=${title}`);
    }

    async clickPageByTitle(title: string) {
        const pageNode = await this.findPageByTitle(title);
        await pageNode.click();
        // wait 2000 ms to ensure the page has loaded
        await this.page.waitForTimeout(2000);
    }

    async expectNumberOfTreeNodes(expectedCount: number) {
        const actualCount = await this.getNumberOfTreeNodes();
        expect(actualCount).toBe(expectedCount);
    }
}
