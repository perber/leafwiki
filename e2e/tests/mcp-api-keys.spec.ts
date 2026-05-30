import { Page, expect, test } from '@playwright/test';
import LoginPage from '../pages/LoginPage';
import { toAppPath } from '../pages/appPath';
import { connectMCPClient } from './mcpClient';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

test.skip(
  process.env.E2E_RUN_MODE !== 'local' || process.env.E2E_ENABLE_MCP_API_KEYS_LOCAL !== '1',
  'Set E2E_RUN_MODE=local and E2E_ENABLE_MCP_API_KEYS_LOCAL=1 to run the MCP API-key smoke test.',
);

function appURL(path: string): string {
  return new URL(toAppPath(path), process.env.E2E_BASE_URL || 'http://localhost:8080').toString();
}

type TestUser = {
  id: string;
  username: string;
  email: string;
  role: 'admin' | 'editor' | 'viewer';
  password: string;
};

async function loginAsAdmin(page: Page) {
  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login(user, password);
  await expect(page).not.toHaveURL(/\/login/);
}

async function csrfHeaders(page: Page): Promise<Record<string, string>> {
  const cookies = await page.context().cookies(appURL('/'));
  const csrf = cookies.find(
    (cookie) => cookie.name === '__Host-leafwiki_csrf' || cookie.name === 'leafwiki_csrf',
  );
  return csrf ? { 'X-CSRF-Token': csrf.value } : {};
}

async function createUserViaAPI(page: Page, role: TestUser['role']): Promise<TestUser> {
  const suffix = `${Date.now()}${Math.floor(Math.random() * 100000)}`;
  const testUser = {
    username: `${role}e2e${suffix}`,
    email: `${role}e2e${suffix}@example.com`,
    password: `${role}pass${suffix}`,
    role,
  };
  const response = await page.request.post(appURL('/api/users'), {
    data: testUser,
    headers: await csrfHeaders(page),
  });
  const body = await response.text();
  expect(response.status(), body).toBe(201);
  return { ...(JSON.parse(body) as Omit<TestUser, 'password'>), password: testUser.password };
}

async function openSelfAPIKeysDialog(page: Page) {
  await page.getByTestId('user-toolbar-avatar').click();
  await page.getByRole('menuitem', { name: 'MCP API Keys' }).click();
  await expect(page.getByRole('heading', { name: 'MCP API Keys' })).toBeVisible();
}

async function openAdminAPIKeysDialog(page: Page, username: string) {
  await page.goto(appURL('/users'));
  const row = page.getByRole('row', { name: new RegExp(username) });
  await expect(row).toBeVisible();
  await row.getByRole('button', { name: 'MCP API Keys' }).click();
  await expect(page.getByRole('heading', { name: `MCP API Keys: ${username}` })).toBeVisible();
}

test('self-service api key works with the official mcp typescript client and fails after revoke', async ({
  page,
}) => {
  await loginAsAdmin(page);
  await openSelfAPIKeysDialog(page);

  const keyName = `E2E self key ${Date.now()}`;
  await page.getByTestId('mcp-api-keys-dialog-name-input').fill(keyName);
  await page.getByTestId('mcp-api-keys-dialog-current-password-input').fill(password);
  await page.getByTestId('mcp-api-keys-dialog-button-create').click();

  const secretInput = page.getByTestId('mcp-api-keys-dialog-secret-input');
  await expect(secretInput).toBeVisible();
  await expect(page.getByTestId('mcp-api-keys-dialog-copy-secret')).toBeVisible();
  const apiKey = await secretInput.inputValue();
  expect(apiKey).toMatch(/^lwk_/);

  await page.getByTestId('mcp-api-keys-dialog-name-input').fill('x'.repeat(81));
  await page.getByTestId('mcp-api-keys-dialog-current-password-input').fill(password);
  await page.getByTestId('mcp-api-keys-dialog-button-create').click();
  await expect(page.getByText('Name must be at most 80 characters long')).toBeVisible();
  await expect(secretInput).toBeVisible();
  await expect(secretInput).toHaveValue(apiKey);

  await page.getByTestId('mcp-api-keys-dialog-button-cancel').click();
  await openSelfAPIKeysDialog(page);
  await expect(page.getByTestId('mcp-api-keys-dialog-secret-input')).toHaveCount(0);

  const mcp = await connectMCPClient(appURL('/mcp'), {
    accessToken: apiKey,
    clientName: 'leafwiki-e2e-api-key',
  });
  try {
    const tools = await mcp.listTools();
    expect(tools).toContain('get_current_user');
    expect(tools).toContain('create_page');
    expect(tools).toContain('get_page');

    const current = await mcp.callTool('get_current_user');
    const currentUser = current.user as { username: string; role: string };
    expect(currentUser.username).toBe(user);
    expect(currentUser.role).toBe('admin');

    const slug = `mcp-api-key-e2e-${Date.now()}`;
    const created = await mcp.callTool('create_page', {
      title: 'MCP API Key E2E Page',
      slug,
      kind: 'page',
    });
    const createdPage = created.page as { id: string; content: string };
    expect(createdPage.id).toBeTruthy();

    await page.goto(appURL(`/${slug}`));
    await page.locator('article').waitFor({ state: 'visible' });
    await expect(page.locator('article')).toContainText('MCP API Key E2E Page');
  } finally {
    await mcp.close();
  }

  await openSelfAPIKeysDialog(page);
  await page
    .locator('[data-testid^="mcp-api-key-row-"]')
    .filter({ hasText: keyName })
    .getByRole('button', { name: `Revoke API key ${keyName}` })
    .click();
  await expect(page.getByText('No active keys.')).toBeVisible();
  await expect(
    connectMCPClient(appURL('/mcp'), {
      accessToken: apiKey,
      clientName: 'leafwiki-e2e-api-key-revoked',
    }),
  ).rejects.toThrow();
});

test('self-service dialog keeps the copy-once secret reachable when close is attempted during create', async ({
  page,
}) => {
  await loginAsAdmin(page);

  let releaseCreate: () => void = () => {};
  const createReleased = new Promise<void>((resolve) => {
    releaseCreate = resolve;
  });
  let markCreateStarted: () => void = () => {};
  const createStarted = new Promise<void>((resolve) => {
    markCreateStarted = resolve;
  });
  await page.route('**/api/users/me/mcp-api-keys', async (route) => {
    if (route.request().method() !== 'POST') {
      await route.fallback();
      return;
    }
    markCreateStarted();
    await createReleased;
    await route.fallback();
  });

  await openSelfAPIKeysDialog(page);
  await page.getByTestId('mcp-api-keys-dialog-name-input').fill(`E2E pending key ${Date.now()}`);
  await page.getByTestId('mcp-api-keys-dialog-current-password-input').fill(password);
  await page.getByTestId('mcp-api-keys-dialog-button-create').click();
  await createStarted;

  await page.keyboard.press('Escape');
  await expect(page.getByRole('heading', { name: 'MCP API Keys' })).toBeVisible();
  await page.getByRole('button', { name: 'Close' }).last().click();
  await expect(page.getByRole('heading', { name: 'MCP API Keys' })).toBeVisible();

  releaseCreate();
  const secretInput = page.getByTestId('mcp-api-keys-dialog-secret-input');
  await expect(secretInput).toBeVisible();
  await expect(secretInput).toHaveValue(/^lwk_/);
});

test('self-service dialog shows retry state instead of an empty list after load failure', async ({
  page,
}) => {
  await loginAsAdmin(page);

  let listAttempts = 0;
  await page.route('**/api/users/me/mcp-api-keys', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    listAttempts += 1;
    if (listAttempts === 1) {
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'temporary failure' }),
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: '[]',
    });
  });

  await openSelfAPIKeysDialog(page);
  await expect(page.getByText('Could not load API keys.')).toBeVisible();
  await expect(page.getByText('No active keys.')).toHaveCount(0);

  await page.getByTestId('mcp-api-keys-dialog-name-input').fill('Retry key');
  await page.getByTestId('mcp-api-keys-dialog-current-password-input').fill(password);
  await expect(page.getByTestId('mcp-api-keys-dialog-button-create')).toBeDisabled();

  await page.getByRole('button', { name: 'Retry' }).click();
  await expect(page.getByText('No active keys.')).toBeVisible();
  await expect(page.getByText('Could not load API keys.')).toHaveCount(0);
  await expect(page.getByTestId('mcp-api-keys-dialog-button-create')).toBeEnabled();
});

test('self-service dialog reports a wrong current password without showing a secret', async ({
  page,
}) => {
  await loginAsAdmin(page);
  await openSelfAPIKeysDialog(page);

  await page.getByTestId('mcp-api-keys-dialog-name-input').fill(`E2E wrong password ${Date.now()}`);
  await page.getByTestId('mcp-api-keys-dialog-current-password-input').fill('wrong-password');
  await page.getByTestId('mcp-api-keys-dialog-button-create').click();

  await expect(page.getByText('Current password is incorrect')).toBeVisible();
  await expect(page.getByTestId('mcp-api-keys-dialog-secret-input')).toHaveCount(0);
  await expect(page.getByRole('heading', { name: 'MCP API Keys' })).toBeVisible();
});

test('remote-user mode disables self-service key creation UI', async ({ page }) => {
  await page.route('**/api/config', async (route) => {
    const response = await route.fetch();
    const config = (await response.json()) as Record<string, unknown>;
    await route.fulfill({
      response,
      json: { ...config, httpRemoteUserEnabled: true },
    });
  });

  await loginAsAdmin(page);
  await openSelfAPIKeysDialog(page);

  await expect(
    page.getByText('Creating keys is unavailable for HTTP remote-user sign-in.'),
  ).toBeVisible();
  await expect(page.getByTestId('mcp-api-keys-dialog-name-input')).toHaveCount(0);
  await expect(page.getByTestId('mcp-api-keys-dialog-button-create')).toHaveCount(0);
});

test('admin-created viewer api key can read through mcp but cannot mutate and can be revoked', async ({
  page,
}) => {
  await loginAsAdmin(page);
  const viewer = await createUserViaAPI(page, 'viewer');
  await openAdminAPIKeysDialog(page, viewer.username);

  const keyName = `E2E viewer key ${Date.now()}`;
  await page.getByTestId('mcp-api-keys-dialog-name-input').fill(keyName);
  await page.getByTestId('mcp-api-keys-dialog-button-create').click();

  const secretInput = page.getByTestId('mcp-api-keys-dialog-secret-input');
  await expect(secretInput).toBeVisible();
  const apiKey = await secretInput.inputValue();
  expect(apiKey).toMatch(/^lwk_/);

  await page.getByTestId('mcp-api-keys-dialog-button-cancel').click();
  await openAdminAPIKeysDialog(page, viewer.username);
  await expect(page.getByTestId('mcp-api-keys-dialog-secret-input')).toHaveCount(0);

  const mcp = await connectMCPClient(appURL('/mcp'), {
    accessToken: apiKey,
    clientName: 'leafwiki-e2e-viewer-api-key',
  });
  try {
    await expect(mcp.callTool('get_tree')).resolves.toBeTruthy();
    await expect(
      mcp.callTool('create_page', {
        title: 'Viewer API Key Write',
        slug: `viewer-api-key-write-${Date.now()}`,
      }),
    ).rejects.toThrow(/editor|admin/i);
  } finally {
    await mcp.close();
  }

  await page
    .locator('[data-testid^="mcp-api-key-row-"]')
    .filter({ hasText: keyName })
    .getByRole('button', { name: `Revoke API key ${keyName}` })
    .click();
  await expect(page.getByText('No active keys.')).toBeVisible();
  await expect(
    connectMCPClient(appURL('/mcp'), {
      accessToken: apiKey,
      clientName: 'leafwiki-e2e-viewer-api-key-revoked',
    }),
  ).rejects.toThrow();
});

test('admin api key dialog validates names from user management', async ({ page }) => {
  await loginAsAdmin(page);
  await page.goto(appURL('/users'));

  const adminRow = page.getByRole('row', { name: new RegExp(user) });
  await adminRow.getByRole('button', { name: 'MCP API Keys' }).click();
  await expect(page.getByRole('heading', { name: /MCP API Keys:/ })).toBeVisible();

  await page.getByTestId('mcp-api-keys-dialog-name-input').fill('x'.repeat(81));
  await page.getByTestId('mcp-api-keys-dialog-button-create').click();
  await expect(page.getByText('Name must be at most 80 characters long')).toBeVisible();
  await expect(page.getByRole('heading', { name: /MCP API Keys:/ })).toBeVisible();
});

test('api key create stays disabled while the key list is loading', async ({ page }) => {
  await loginAsAdmin(page);

  let releaseList: () => void = () => {};
  const listReleased = new Promise<void>((resolve) => {
    releaseList = resolve;
  });
  await page.route('**/api/users/me/mcp-api-keys', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await listReleased;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: '[]',
    });
  });

  await page.getByTestId('user-toolbar-avatar').click();
  await page.getByRole('menuitem', { name: 'MCP API Keys' }).click();
  await expect(page.getByRole('heading', { name: 'MCP API Keys' })).toBeVisible();

  await page.getByTestId('mcp-api-keys-dialog-name-input').fill('Race key');
  await page.getByTestId('mcp-api-keys-dialog-current-password-input').fill(password);
  await expect(page.getByTestId('mcp-api-keys-dialog-button-create')).toBeDisabled();

  releaseList();
  await expect(page.getByText('No active keys.')).toBeVisible();
  await expect(page.getByTestId('mcp-api-keys-dialog-button-create')).toBeEnabled();
});
