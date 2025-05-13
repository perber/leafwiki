import { expect, test } from '@playwright/test';

const user = "admin";
//const password = "gD$Z#xcvQtJEU6SS"
const password = "admin"

const login = async (page) => {
    await page.goto('/login');
    await page.getByRole('textbox', { name: 'Username or Email' }).click();
    await page.getByRole('textbox', { name: 'Username or Email' }).fill(user);
    await page.getByRole('textbox', { name: 'Username or Email' }).press('Tab');
    await page.getByRole('textbox', { name: 'Password' }).click();
    await page.getByRole('textbox', { name: 'Password' }).fill(password);
    await page.getByRole('button', { name: 'Login' }).click();
}

test('failed login', async ({ page }) => {
    await page.goto('/login');
    await page.getByRole('textbox', { name: 'Username or Email' }).click();
    await page.getByRole('textbox', { name: 'Username or Email' }).fill(user);
    await page.getByRole('textbox', { name: 'Username or Email' }).press('Tab');
    await page.getByRole('textbox', { name: 'Password' }).fill('failed');
    await page.getByRole('textbox', { name: 'Password' }).press('Enter');
    await page.getByRole('button', { name: 'Login' }).click();
    await expect(page.getByRole('paragraph')).toContainText('Invalid credentials');
  });

  test('successful login', async ({ page }) => {
    await login(page);
    await expect(page.getByRole('button', { name: 'A' })).toBeVisible();
  });

  test('create page', async ({ page }) => {
    await login(page);
    await page.getByRole('button').first().click();
    await page.getByRole('textbox', { name: 'Page title' }).click();
    await page.getByRole('textbox', { name: 'Page title' }).fill('Eine neue Seite');
    await page.getByRole('button', { name: 'Create' }).click();
    await expect(page.getByText('eine-neue-seite')).toBeVisible();
    await page.getByRole('textbox', { name: 'Write in Markdown...' }).click();
    await page.getByRole('textbox', { name: 'Write in Markdown...' }).fill('# Eine neue Seite\n\nEin neuer Test kommt hinzu');
    await page.getByRole('button').filter({ hasText: /^$/ }).nth(3).click();
    await page.getByRole('button').filter({ hasText: /^$/ }).nth(2).click();
    await expect(page.getByRole('paragraph')).toContainText('Ein neuer Test kommt hinzu');
  });

