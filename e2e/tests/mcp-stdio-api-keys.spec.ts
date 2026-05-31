import { Page, expect, test } from '@playwright/test';
import LoginPage from '../pages/LoginPage';
import { toAppPath } from '../pages/appPath';
import { connectMCPStdioClient, requestMCPStdioFrame } from './mcpClient';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

test.skip(
  process.env.E2E_RUN_MODE !== 'local' ||
    process.env.E2E_ENABLE_MCP_API_KEYS_LOCAL !== '1' ||
    process.env.E2E_MCP_CLIENT_TRANSPORT !== 'stdio',
  'Set E2E_RUN_MODE=local, E2E_ENABLE_MCP_API_KEYS_LOCAL=1, and E2E_MCP_CLIENT_TRANSPORT=stdio to run the MCP stdio API-key smoke test.',
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
    username: `${role}stdio${suffix}`,
    email: `${role}stdio${suffix}@example.com`,
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

async function deleteUserViaAPI(page: Page, userID: string) {
  const response = await page.request.delete(appURL(`/api/users/${userID}`), {
    headers: await csrfHeaders(page),
  });
  expect(response.status(), await response.text()).toBe(204);
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

async function expectRawStdioUnauthorized(apiKey: string) {
  const result = await requestMCPStdioFrame(
    appURL('/mcp'),
    {
      jsonrpc: '2.0',
      id: 1,
      method: 'initialize',
      params: {
        clientInfo: { name: 'leafwiki-e2e-stdio-unauthorized', version: 'test' },
        protocolVersion: '2025-11-25',
      },
    },
    { accessToken: apiKey },
  );
  expect(result.exitCode).toBe(0);
  expect(result.signal).toBeNull();
  expect(result.stdoutLines).toHaveLength(1);
  expect(result.responses).toHaveLength(1);
  expect(result.stdout).not.toContain(apiKey);
  expect(result.stderr).not.toContain(apiKey);
  expect(result.response?.error?.code).toBe(-32000);
  expect([401, 403]).toContain(result.response?.error?.data?.status);
  expect(result.stderr).toMatch(/HTTP (401|403)/);
}

async function expectSDKStdioConnectRejected(apiKey: string, clientName: string) {
  let timeout: NodeJS.Timeout | undefined;
  let connected = false;
  try {
    const client = await Promise.race([
      connectMCPStdioClient(appURL('/mcp'), {
        accessToken: apiKey,
        clientName,
      }),
      new Promise<never>((_, reject) => {
        timeout = setTimeout(() => {
          reject(new Error('timed out waiting for SDK stdio connect rejection'));
        }, 5000);
      }),
    ]);
    connected = true;
    await client.close();
  } catch (error) {
    if (error instanceof Error && error.message.includes('timed out')) {
      throw error;
    }
    if (connected) {
      throw error;
    }
    return;
  } finally {
    if (timeout) {
      clearTimeout(timeout);
    }
  }
  throw new Error('SDK stdio connect unexpectedly succeeded');
}

test('self-service api key works through the stdio sidecar and fails after revoke', async ({
  page,
}) => {
  await loginAsAdmin(page);
  await openSelfAPIKeysDialog(page);

  const keyName = `E2E stdio self key ${Date.now()}`;
  await page.getByTestId('mcp-api-keys-dialog-name-input').fill(keyName);
  await page.getByTestId('mcp-api-keys-dialog-current-password-input').fill(password);
  await page.getByTestId('mcp-api-keys-dialog-button-create').click();

  const secretInput = page.getByTestId('mcp-api-keys-dialog-secret-input');
  await expect(secretInput).toBeVisible();
  const apiKey = await secretInput.inputValue();
  expect(apiKey).toMatch(/^lwk_/);

  const mcp = await connectMCPStdioClient(appURL('/mcp'), {
    accessToken: apiKey,
    clientName: 'leafwiki-e2e-stdio-api-key',
  });
  try {
    const current = await mcp.callTool('get_current_user');
    const currentUser = current.user as { username: string; role: string };
    expect(currentUser.username).toBe(user);
    expect(currentUser.role).toBe('admin');

    const slug = `mcp-stdio-api-key-e2e-${Date.now()}`;
    const created = await mcp.callTool('create_page', {
      title: 'MCP STDIO API Key E2E Page',
      slug,
      kind: 'page',
    });
    const createdPage = created.page as { id: string };
    expect(createdPage.id).toBeTruthy();

    await page.goto(appURL(`/${slug}`));
    await page.locator('article').waitFor({ state: 'visible' });
    await expect(page.locator('article')).toContainText('MCP STDIO API Key E2E Page');
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
  await expectRawStdioUnauthorized(apiKey);
  await expectSDKStdioConnectRejected(apiKey, 'leafwiki-e2e-stdio-api-key-revoked');
});

test('viewer api key can read through stdio but cannot mutate and can be revoked', async ({
  page,
}) => {
  await loginAsAdmin(page);
  const viewer = await createUserViaAPI(page, 'viewer');
  await openAdminAPIKeysDialog(page, viewer.username);

  const keyName = `E2E stdio viewer key ${Date.now()}`;
  await page.getByTestId('mcp-api-keys-dialog-name-input').fill(keyName);
  await page.getByTestId('mcp-api-keys-dialog-button-create').click();

  const secretInput = page.getByTestId('mcp-api-keys-dialog-secret-input');
  await expect(secretInput).toBeVisible();
  const apiKey = await secretInput.inputValue();
  expect(apiKey).toMatch(/^lwk_/);

  await page.getByTestId('mcp-api-keys-dialog-button-cancel').click();
  await openAdminAPIKeysDialog(page, viewer.username);
  await expect(page.getByTestId('mcp-api-keys-dialog-secret-input')).toHaveCount(0);

  const mcp = await connectMCPStdioClient(appURL('/mcp'), {
    accessToken: apiKey,
    clientName: 'leafwiki-e2e-stdio-viewer-api-key',
  });
  try {
    await expect(mcp.callTool('get_tree')).resolves.toBeTruthy();
    await expect(
      mcp.callTool('create_page', {
        title: 'Viewer STDIO API Key Write',
        slug: `viewer-stdio-api-key-write-${Date.now()}`,
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
  await expectRawStdioUnauthorized(apiKey);
  await expectSDKStdioConnectRejected(apiKey, 'leafwiki-e2e-stdio-viewer-api-key-revoked');
});

test('deleted-user api key fails through the stdio sidecar', async ({ page }) => {
  await loginAsAdmin(page);
  const editor = await createUserViaAPI(page, 'editor');
  await openAdminAPIKeysDialog(page, editor.username);

  await page
    .getByTestId('mcp-api-keys-dialog-name-input')
    .fill(`E2E stdio deleted-user key ${Date.now()}`);
  await page.getByTestId('mcp-api-keys-dialog-button-create').click();

  const secretInput = page.getByTestId('mcp-api-keys-dialog-secret-input');
  await expect(secretInput).toBeVisible();
  const apiKey = await secretInput.inputValue();
  expect(apiKey).toMatch(/^lwk_/);

  await page.getByTestId('mcp-api-keys-dialog-button-cancel').click();
  await deleteUserViaAPI(page, editor.id);

  await expectRawStdioUnauthorized(apiKey);
  await expectSDKStdioConnectRejected(apiKey, 'leafwiki-e2e-stdio-deleted-user-api-key');
});
