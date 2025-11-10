import { expect, Page } from '@playwright/test';
import AddPageDialog from './AddPageDialog';
import MovePageDialog from './MovePageDialog';
import SortPageDialog from './SortPageDialog';

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

    async expandNodeByTitle(title: string) {
        const nodeRow = this.page
            .locator('div[data-testid^="tree-node-"]')
            .filter({ hasText: title })
            .first();

        await nodeRow.scrollIntoViewIfNeeded();
        await nodeRow.hover();

        const toggleIcon = nodeRow.locator('svg[data-testid^="tree-node-toggle-icon-"]');
        if (await toggleIcon.isVisible()) {
            await toggleIcon.click({ force: true });
        }
    }

    async createSubPageOfParent(parentTitle: string, newSubpageTitle: string) {
        const nodeRow = this.page
            .locator('div[data-testid^="tree-node-"]')
            .filter({ hasText: parentTitle })
            .first();

        await nodeRow.scrollIntoViewIfNeeded();
        await nodeRow.hover(); // oder mouse.move, s.u.

        const addButton = nodeRow.locator('button[data-testid="tree-view-action-button-add"]');
        await addButton.click({ force: true });

        const addPageDialog = new AddPageDialog(this.page);
        await addPageDialog.fillTitle(newSubpageTitle);
        await addPageDialog.submitWithoutRedirect();
    }

    async createMultipleSubPagesOfParent(parentTitle: string, subpageTitles: string[]) {
        if (!(await this.isSidebarVisible())) {
            await this.page.getByTestId('sidebar-toggle-button').click();
        }
        for (const title of subpageTitles) {
            await this.createSubPageOfParent(parentTitle, title);
        }
    }

    async sortPagesOfParent(parentTitle: string, plannedOrder: string[]) {
        const nodeRow = this.page
            .locator('div[data-testid^="tree-node-"]')
            .filter({ hasText: parentTitle })
            .first();

        await nodeRow.scrollIntoViewIfNeeded();
        await nodeRow.hover(); // oder mouse.move, s.u.

        const sortButton = nodeRow.locator('button[data-testid="tree-view-action-button-sort"]');
        await sortButton.click({ force: true });

        const sortPageDialog = new SortPageDialog(this.page);
        await sortPageDialog.sortPageItems(plannedOrder);

        const orderInDialog = await sortPageDialog.getCurrentOrder();

        expect(orderInDialog).toEqual(plannedOrder);

        await sortPageDialog.saveSorting();
        await this.page.waitForTimeout(5000); // wait for sorting to be applied
    }

    async movePageToTopLevel(parentPage: string, pageTitle: string) {

        await this.expandNodeByTitle(parentPage);

        const nodeRow = this.page
            .locator('div[data-testid^="tree-node-"]')
            .filter({ hasText: pageTitle })
            .first();

        await nodeRow.scrollIntoViewIfNeeded();
        await nodeRow.hover(); // oder mouse.move, s.u.

        const moveButton = nodeRow.locator('button[data-testid="tree-view-action-button-move"]');
        await moveButton.click({ force: true });

        const movePageDialog = new MovePageDialog(this.page);
        await movePageDialog.selectNewParentAsTopLevel();
        await movePageDialog.clickMoveButton();

        await this.page.waitForTimeout(5000); // wait for move to be applied
    }

    async expectNumberOfTreeNodes(expectedCount: number) {
        const actualCount = await this.getNumberOfTreeNodes();
        expect(actualCount).toBe(expectedCount);
    }
}
