import { expect, Page } from '@playwright/test';
import AddPageDialog from './AddPageDialog';
import MovePageDialog from './MovePageDialog';
import SortPageDialog from './SortPageDialog';

export default class TreeView {
  constructor(private page: Page) {}

  private async closeBlockingOverlayIfPresent() {
    const overlay = this.page.locator('div[data-state="open"][data-aria-hidden="true"]').last();

    try {
      if (!(await overlay.isVisible({ timeout: 500 }))) {
        return;
      }
    } catch {
      return;
    }

    await this.page.keyboard.press('Escape');
    await overlay.waitFor({ state: 'hidden', timeout: 5000 });
  }

  private treeView() {
    return this.page.locator('.tree-view:visible').first();
  }

  private async ensureSidebarVisible() {
    if (!(await this.isSidebarVisible())) {
      await this.page.getByTestId('sidebar-toggle-button').click();
    }
  }

  private getNodeRowByTitle(title: string) {
    const pageLink = this.page.locator('a[data-testid^="tree-node-link-"]', {
      hasText: title,
    });
    return this.page.locator('div[data-testid^="tree-node-"]').filter({ has: pageLink }).first();
  }

  private async openMoreActionsMenuForNodeRow(
    nodeRow: ReturnType<TreeView['getNodeRowByTitle']>,
    expectedActionTestId: 'tree-view-action-button-sort' | 'tree-view-action-button-move',
  ) {
    await nodeRow.waitFor({ state: 'visible' });
    await nodeRow.scrollIntoViewIfNeeded();
    await nodeRow.hover();

    const moreActionsButton = nodeRow.locator(
      'button[data-testid="tree-view-action-button-open-more-actions"]',
    );
    await expect(moreActionsButton).toBeVisible();
    const expectedActionButton = this.page.locator(
      `[role="menu"] [data-testid="${expectedActionTestId}"]`,
    );

    for (let attempt = 0; attempt < 3; attempt += 1) {
      await nodeRow.hover();
      await moreActionsButton.waitFor({ state: 'visible' });

      try {
        await moreActionsButton.click({ timeout: 2000 });
      } catch {
        await moreActionsButton.click({ force: true });
      }

      try {
        await expect(expectedActionButton).toBeVisible({ timeout: 2000 });
        return;
      } catch {
        await this.page.keyboard.press('Escape').catch(() => {});
      }
    }

    await expect(expectedActionButton).toBeVisible();
  }

  async getRootAddButton() {
    return this.page.getByTestId('sidebar').getByTestId('tree-view-action-button-add').first();
  }

  async clickRootAddButton() {
    await this.ensureSidebarVisible();
    await (await this.getRootAddButton()).click();
  }

  async isSidebarVisible(): Promise<boolean> {
    return this.page.getByTestId('sidebar').isVisible();
  }

  async getNumberOfTreeNodes() {
    await this.page.waitForLoadState('networkidle');
    await this.ensureSidebarVisible();
    return this.treeView().locator('a[data-testid^="tree-node-link-"]').count();
  }

  async findPageByTitle(title: string) {
    return this.page
      .locator('a[data-testid^="tree-node-link-"]', {
        hasText: title,
      })
      .first();
  }

  async expectPageHighlighted(title: string) {
    const pageNode = await this.findPageByTitle(title);
    await expect(pageNode).toHaveAttribute('aria-current', 'page');
  }

  async clickPageByTitle(title: string) {
    await this.ensureSidebarVisible();
    await this.closeBlockingOverlayIfPresent();
    const pageNode = await this.findPageByTitle(title);
    const href = await pageNode.getAttribute('href');
    await pageNode.waitFor({ state: 'visible' });
    await pageNode.click();
    await expect(pageNode).toHaveAttribute('aria-current', 'page');
    if (href) {
      const expectedPath = new URL(href, 'http://localhost').pathname;
      await expect.poll(() => new URL(this.page.url()).pathname).toBe(expectedPath);
    }
    await this.page.locator('article').waitFor({ state: 'visible' });
  }

  async expandNodeByTitle(title: string) {
    await this.ensureSidebarVisible();
    await this.closeBlockingOverlayIfPresent();
    const nodeRow = this.getNodeRowByTitle(title);

    await nodeRow.waitFor({ state: 'visible' });
    await nodeRow.scrollIntoViewIfNeeded();
    await nodeRow.hover();

    const toggleIcon = nodeRow.locator('svg[data-testid^="tree-node-toggle-icon-"]');
    if (await toggleIcon.isVisible()) {
      const classes = (await toggleIcon.getAttribute('class')) || '';
      if (!classes.includes('tree-node__toggle--open')) {
        await toggleIcon.click({ force: true });
      }
    }
  }

  async createSubPageOfParent(parentTitle: string, newSubpageTitle: string) {
    await this.ensureSidebarVisible();
    await this.closeBlockingOverlayIfPresent();
    const nodeRow = this.getNodeRowByTitle(parentTitle);

    await nodeRow.waitFor({ state: 'visible' });
    await nodeRow.scrollIntoViewIfNeeded();
    await nodeRow.hover(); // oder mouse.move, s.u.

    const addButton = nodeRow.locator('button[data-testid="tree-view-action-button-add"]');
    await addButton.click({ force: true });

    const addPageDialog = new AddPageDialog(this.page);
    await addPageDialog.fillTitle(newSubpageTitle);
    await addPageDialog.submitWithoutRedirect();
    await this.page.waitForLoadState('networkidle');
  }

  async createMultipleSubPagesOfParent(parentTitle: string, subpageTitles: string[]) {
    await this.ensureSidebarVisible();
    for (const title of subpageTitles) {
      await this.createSubPageOfParent(parentTitle, title);
    }
  }

  async sortPagesOfParent(parentTitle: string, plannedOrder: string[]) {
    await this.ensureSidebarVisible();
    await this.closeBlockingOverlayIfPresent();
    await this.expandNodeByTitle(parentTitle);

    for (const title of plannedOrder) {
      await expect(await this.findPageByTitle(title)).toBeVisible();
    }

    const nodeRow = this.getNodeRowByTitle(parentTitle);

    await this.openMoreActionsMenuForNodeRow(nodeRow, 'tree-view-action-button-sort');

    const sortButton = this.page.locator(
      '[role="menu"] [data-testid="tree-view-action-button-sort"]',
    );
    await expect(sortButton).toBeVisible();
    await sortButton.click();

    const sortPageDialog = new SortPageDialog(this.page);
    await sortPageDialog.sortPageItems(plannedOrder);

    const orderInDialog = await sortPageDialog.getCurrentOrder();

    expect(orderInDialog).toEqual(plannedOrder);

    await sortPageDialog.saveSorting();
    await this.page.waitForLoadState('networkidle');
  }

  async movePageToTopLevel(parentPage: string, pageTitle: string) {
    await this.openMoveDialogForPage(parentPage, pageTitle);

    const movePageDialog = new MovePageDialog(this.page);
    await movePageDialog.selectNewParentAsTopLevel();
    await movePageDialog.clickMoveButton();
    await this.page.waitForLoadState('networkidle');
  }

  async openMoveDialogForPage(parentPage: string, pageTitle: string) {
    await this.ensureSidebarVisible();
    await this.closeBlockingOverlayIfPresent();
    await this.expandNodeByTitle(parentPage);

    const nodeRow = this.getNodeRowByTitle(pageTitle);
    await this.openMoreActionsMenuForNodeRow(nodeRow, 'tree-view-action-button-move');

    const moveButton = this.page.getByTestId('tree-view-action-button-move');
    await expect(moveButton).toBeVisible();
    await moveButton.click();
  }

  async expectNumberOfTreeNodes(expectedCount: number) {
    await this.page.waitForLoadState('networkidle');
    await expect(this.treeView().locator('a[data-testid^="tree-node-link-"]')).toHaveCount(
      expectedCount,
    );
  }
}
