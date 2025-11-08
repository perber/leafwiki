import { Page } from '@playwright/test';

export default class LoginPage {
    constructor(private page: Page) { }

    async getTitleInput() {
        return this.page.locator('input[data-testid="add-page-title-input"]');
    }

    async getSlugInput() {
        return this.page.locator('input[data-testid="add-page-slug-input"]');
    }

    async getCreateButton() {
        return this.page.locator('button[data-testid="add-page-create-button-without-redirect"]');
    }

    async fillTitle(title: string) {
        const titleInput = await this.getTitleInput();
        const slugInput = await this.getSlugInput();

        await titleInput.fill(title);

        // Wait for slug to be generated
        await this.page.waitForTimeout(500);

        // Get the slug and verify it is generated correctly
        const slug = await slugInput.inputValue();
        const expectedSlug = title.toLowerCase().replace(/\s+/g, '-').replace(/[^\w-]/g, '');
        if (slug !== expectedSlug) {
            throw new Error(`Expected slug to be "${expectedSlug}", but got "${slug}"`);
        }
    }

    async submitWithoutRedirect() {
        const createButton = await this.getCreateButton();
        await createButton.click();
        // Wait a 600 ms to ensure the dialog has processed the creation
        await this.page.waitForTimeout(600);
    }
}
