import { expect, test } from '@playwright/test';

const user = "admin";
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


  test('create a page', async ({ page }) => {
    await login(page);
    await page.locator('.btn-treeview').first().click();
    await page.getByRole('textbox', { name: 'Page title' }).click();
    await page.getByRole('textbox', { name: 'Page title' }).fill('Neue Seite anlegen');
    await page.getByRole('button', { name: 'Create' }).click();
    // CodeMirror Ediotr
    await page.locator('.cm-content').click() 
    await page.keyboard.type('# Hello World\n\nThis is LeafWiki.\n\n Und wir legen eine neue Seite an')  // Tippen
    await page.locator('.sticky > div > .inline-flex').first().click();
    await page.getByRole('banner').getByRole('button').nth(2).click();
    await page.locator('div:nth-child(12) > .inline-flex').click();
    await page.getByRole('button', { name: 'Close' }).click();
    await page.getByRole('banner').getByRole('button').nth(1).click();
    await page.getByRole('button', { name: 'Search' }).click();
    await page.getByRole('textbox', { name: 'Search...' }).click();
    await page.getByRole('textbox', { name: 'Search...' }).fill('Neue');
    await page.getByRole('textbox', { name: 'Search...' }).press('Enter');
    await expect(page.getByRole('link', { name: 'Neue Seite anlegen Neue Seite anlegen Hello World This is LeafWiki. Und wir legen eine neue Seite an**** / neue-seite-anlegen', exact: true })).toBeVisible();
  });