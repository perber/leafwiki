import { APIRequestContext, APIResponse, Page, expect, test } from '@playwright/test';
import crypto from 'node:crypto';
import EditPage from '../pages/EditPage';
import LoginPage from '../pages/LoginPage';
import { toAppPath } from '../pages/appPath';
import ViewPage from '../pages/ViewPage';
import { connectMCPClient, startMCPClientSDKOAuthFlow } from './mcpClient';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

test.skip(
  process.env.E2E_RUN_MODE !== 'local' || process.env.E2E_ENABLE_MCP_OAUTH_LOCAL !== '1',
  'Set E2E_RUN_MODE=local and E2E_ENABLE_MCP_OAUTH_LOCAL=1 to run the MCP OAuth smoke test.',
);

function appURL(path: string): string {
  return new URL(toAppPath(path), process.env.E2E_BASE_URL || 'http://localhost:8080').toString();
}

function loopbackAppURL(path: string): string {
  const baseURL = new URL(process.env.E2E_BASE_URL || 'http://localhost:8080');
  baseURL.hostname = '127.0.0.1';
  return new URL(toAppPath(path), baseURL).toString();
}

function base64URL(input: Buffer): string {
  return input.toString('base64').replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '');
}

function pkcePair(): { verifier: string; challenge: string } {
  const verifier = base64URL(crypto.randomBytes(32));
  const challenge = base64URL(crypto.createHash('sha256').update(verifier).digest());
  return { verifier, challenge };
}

async function approvalFieldsFromAuthorizeRedirect(
  page: Page,
  authorize: APIResponse,
  approvalURLFor: (path: string) => string,
  expectedClientLabel: string,
): Promise<Record<string, string>> {
  expect(authorize.status()).toBe(302);
  const location = authorize.headers().location;
  expect(location).toBeTruthy();

  const approvalURL = new URL(location || '', approvalURLFor('/'));
  expect(approvalURL.origin + approvalURL.pathname).toBe(approvalURLFor('/oauth/approve'));

  await page.goto(approvalURL.toString());
  await expect(page.getByRole('heading', { name: /Authorize MCP access/ })).toBeVisible();
  await expect(page.getByText(expectedClientLabel)).toBeVisible();
  await expect(page.getByText('Client ID')).toBeVisible();

  const fields: Record<string, string> = {};
  for (const [name, value] of approvalURL.searchParams.entries()) {
    fields[name] = value;
  }
  expect(fields.approval_token).toBeTruthy();
  return fields;
}

async function loginAsAdmin(page: Page) {
  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login(user, password);
  await expect(page).not.toHaveURL(/\/login/);
}

async function loginAsAdminAt(page: Page, loginURL: string) {
  const loginPage = new LoginPage(page);
  await page.goto(loginURL);
  await loginPage.login(user, password);
  await expect(page).not.toHaveURL(/\/login/);
}

async function exerciseMCPUIRoundTrip(
  page: Page,
  mcp: Awaited<ReturnType<typeof connectMCPClient>>,
  slugPrefix: string,
  pageURL: (path: string) => string = appURL,
) {
  const slug = `${slugPrefix}-${Date.now()}`;
  const targetSlug = `${slug}-target`;
  const title = 'MCP OAuth E2E Page';
  const target = await mcp.callTool('create_page', {
    title: 'MCP OAuth E2E Target',
    slug: targetSlug,
    kind: 'page',
  });
  const targetPage = target.page as { id: string };
  const created = await mcp.callTool('create_page', { title, slug, kind: 'page' });
  const createdPage = created.page as { id: string; version: string };

  await mcp.callTool('update_page', {
    id: createdPage.id,
    version: createdPage.version,
    title,
    slug,
    content: `Seeded through authenticated MCP\n\n[Target](/${targetSlug}) and [Missing](/${slug}-missing)`,
  });

  const viewPage = new ViewPage(page);
  await page.goto(pageURL(`/${slug}`));
  await page.locator('article').waitFor({ state: 'visible' });
  await expect(page.locator('article')).toContainText('Seeded through authenticated MCP');

  await viewPage.clickEditPageButton();
  const editPage = new EditPage(page);
  await editPage.writeContent('\nUpdated from the UI while authenticated');
  await editPage.savePage();
  await editPage.closeEditor();

  const readBack = await mcp.callTool('get_page', { id: createdPage.id });
  const pageFromMCP = readBack.page as { content: string };
  expect(pageFromMCP.content).toContain('Updated from the UI while authenticated');

  const sourceStatus = await mcp.callTool('get_link_status', { id: createdPage.id });
  const sourceStatusValue = sourceStatus.status as {
    counts?: { broken_outgoings?: number };
  };
  expect(readBack.linkStatus).toEqual(sourceStatus.status);
  expect(sourceStatusValue.counts?.broken_outgoings).toBe(1);

  const sourceByPath = await mcp.callTool('get_page_by_path', { path: slug });
  expect(sourceByPath.linkStatus).toEqual(sourceStatus.status);

  const targetStatus = await mcp.callTool('get_link_status', { id: targetPage.id });
  const targetByID = await mcp.callTool('get_page', { id: targetPage.id });
  const targetByPath = await mcp.callTool('get_page_by_path', { path: targetSlug });
  expect(targetByID.linkStatus).toEqual(targetStatus.status);
  expect(targetByPath.linkStatus).toEqual(targetStatus.status);
  const targetStatusValue = targetStatus.status as {
    backlinks?: Array<Record<string, unknown>>;
  };
  expect(targetStatusValue.backlinks ?? []).toEqual(
    expect.arrayContaining([expect.objectContaining({ from_page_id: createdPage.id })]),
  );
}

async function authorizeAndExchange(page: Page, request: APIRequestContext): Promise<string> {
  const { verifier, challenge } = pkcePair();
  const redirectURI = 'http://127.0.0.1:49152/callback';
  const authorizeURL = new URL(appURL('/oauth/authorize'));
  authorizeURL.searchParams.set('client_id', 'leafwiki-local-mcp');
  authorizeURL.searchParams.set('response_type', 'code');
  authorizeURL.searchParams.set('redirect_uri', redirectURI);
  authorizeURL.searchParams.set('scope', 'leafwiki:mcp');
  authorizeURL.searchParams.set('state', 'mcp-oauth-e2e');
  authorizeURL.searchParams.set('resource', appURL('/mcp'));
  authorizeURL.searchParams.set('code_challenge', challenge);
  authorizeURL.searchParams.set('code_challenge_method', 'S256');

  const authorize = await page.context().request.get(authorizeURL.toString(), {
    maxRedirects: 0,
  });
  const approvalFields = await approvalFieldsFromAuthorizeRedirect(
    page,
    authorize,
    appURL,
    'LeafWiki local MCP',
  );
  const approved = await page.context().request.post(appURL('/oauth/authorize'), {
    form: { ...approvalFields, decision: 'approve' },
    maxRedirects: 0,
  });
  expect(approved.status()).toBe(302);

  const location = approved.headers().location;
  expect(location).toBeTruthy();
  const redirect = new URL(location);
  expect(redirect.origin).toBe('http://127.0.0.1:49152');
  expect(redirect.searchParams.get('state')).toBe('mcp-oauth-e2e');
  const code = redirect.searchParams.get('code');
  expect(code).toBeTruthy();

  const token = await request.post(appURL('/oauth/token'), {
    form: {
      grant_type: 'authorization_code',
      code: code || '',
      redirect_uri: redirectURI,
      client_id: 'leafwiki-local-mcp',
      code_verifier: verifier,
    },
  });
  expect(token.ok()).toBeTruthy();
  const tokenBody = (await token.json()) as { access_token?: string; scope?: string };
  expect(tokenBody.scope).toBe('leafwiki:mcp');
  expect(tokenBody.access_token).toBeTruthy();
  return tokenBody.access_token || '';
}

test('mcp oauth creates page and UI edit is readable through mcp', async ({ page, request }) => {
  await loginAsAdmin(page);
  const accessToken = await authorizeAndExchange(page, request);
  const mcp = await connectMCPClient(appURL('/mcp'), {
    accessToken,
    clientName: 'leafwiki-e2e-oauth',
  });

  try {
    const tools = await mcp.listTools();
    expect(tools).toContain('get_current_user');
    expect(tools).toContain('create_page');
    expect(tools).toContain('update_page');
    expect(tools).toContain('get_page');

    const current = await mcp.callTool('get_current_user');
    const currentUser = current.user as { username: string; role: string };
    expect(currentUser.username).toBe(user);
    expect(currentUser.role).toBe('admin');

    await exerciseMCPUIRoundTrip(page, mcp, 'mcp-oauth-e2e');
  } finally {
    await mcp.close();
  }
});

test('mcp oauth sdk dynamically registers, discovers protected resource metadata, and connects', async ({
  page,
}) => {
  const mcpURL = loopbackAppURL('/mcp');
  const redirectURI = 'http://127.0.0.1:49152/callback';
  const oauthFlow = await startMCPClientSDKOAuthFlow(mcpURL, {
    clientName: 'leafwiki-e2e-oauth-discovery',
    dynamicRegistration: true,
    redirectURI,
  });
  const registeredClientID = oauthFlow.clientID();

  expect(oauthFlow.authorizationURL.origin + oauthFlow.authorizationURL.pathname).toBe(
    loopbackAppURL('/oauth/authorize'),
  );
  expect(registeredClientID).toBeTruthy();
  expect(registeredClientID).not.toBe('leafwiki-local-mcp');
  expect(oauthFlow.authorizationURL.searchParams.get('client_id')).toBe(registeredClientID);
  expect(oauthFlow.authorizationURL.searchParams.get('resource')).toBe(mcpURL);

  await loginAsAdminAt(page, loopbackAppURL('/login'));
  const authorize = await page.context().request.get(oauthFlow.authorizationURL.toString(), {
    maxRedirects: 0,
  });
  const approvalFields = await approvalFieldsFromAuthorizeRedirect(
    page,
    authorize,
    loopbackAppURL,
    'leafwiki-e2e-oauth-discovery',
  );
  const approved = await page.context().request.post(loopbackAppURL('/oauth/authorize'), {
    form: { ...approvalFields, decision: 'approve' },
    maxRedirects: 0,
  });
  expect(approved.status()).toBe(302);

  const location = approved.headers().location;
  expect(location).toBeTruthy();
  const redirect = new URL(location);
  expect(redirect.origin).toBe('http://127.0.0.1:49152');
  const code = redirect.searchParams.get('code');
  expect(code).toBeTruthy();

  const mcp = await oauthFlow.finishAuth(code || '');
  try {
    const current = await mcp.callTool('get_current_user');
    const currentUser = current.user as { username: string; role: string };
    expect(currentUser.username).toBe(user);
    expect(currentUser.role).toBe('admin');
    expect(await mcp.listTools()).toContain('get_page');
    await exerciseMCPUIRoundTrip(page, mcp, 'mcp-oauth-dcr-e2e', loopbackAppURL);
  } finally {
    await mcp.close();
  }
});
