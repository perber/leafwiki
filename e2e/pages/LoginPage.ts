import { expect, Page } from '@playwright/test';

export default class LoginPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/login');
  }

  async getUsernameInput() {
    return this.page.locator('input[data-testid="login-identifier"]');
  }

  async getPasswordInput() {
    return this.page.locator('input[data-testid="login-password"]');
  }

  async getSubmitButton() {
    return this.page.locator('button[data-testid="login-submit"]');
  }

  async expectInvalidCredentialsError() {
    await expect(this.page.getByText('Invalid credentials')).toBeVisible();
  }

  async login(identifier: string, password: string) {
    const identifierInput = await this.getUsernameInput();
    const passwordInput = await this.getPasswordInput();
    const submitBtn = await this.getSubmitButton();

    await identifierInput.fill(identifier);
    await passwordInput.fill(password);
    await submitBtn.click();
  }
}
