import { Page } from '@playwright/test';

export default class MovePageDialog {
    constructor(private page: Page) { }

    async getParentSelection() {
        // <button type="button" role="combobox" aria-controls="radix-_r_638_" aria-expanded="false" aria-autocomplete="none" dir="ltr" data-state="closed" class="border-input ring-offset-background focus:ring-ring data-placeholder:text-muted-foreground flex h-9 w-full items-center justify-between rounded-md border bg-transparent px-3 py-2 text-sm whitespace-nowrap shadow-xs focus:ring-1 focus:outline-hidden disabled:cursor-not-allowed disabled:opacity-50 [&amp;>span]:line-clamp-1"><span style="pointer-events: none;">— mermaid</span><svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-chevron-down h-4 w-4 opacity-50" aria-hidden="true"><path d="m6 9 6 6 6-6"></path></svg></button>
        return this.page.locator('button[role="combobox"]');
    }

    async selectNewParentAsTopLevel() {
        const parentSelection = await this.getParentSelection();
        await parentSelection.click();
        // find by text contains "Top Level" should be regex because at the beginning there is an emoji
        const option = this.page.locator(`div[role="option"]`).filter({ hasText: new RegExp("Top Level") }).first();
        await option.click();
    }

    async clickMoveButton() {
        const moveButton = this.page.locator('button[data-testid="move-page-dialog-save-button"]');
        await moveButton.click();
    }
}
