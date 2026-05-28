import { APIRequestContext, expect, test } from '@playwright/test';
import EditPage from '../pages/EditPage';
import ViewPage from '../pages/ViewPage';

type MCPClient = {
  callTool: (name: string, args?: Record<string, unknown>) => Promise<Record<string, unknown>>;
};

test.skip(
  process.env.E2E_RUN_MODE !== 'local' || process.env.E2E_ENABLE_MCP_LOCAL !== '1',
  'Set E2E_RUN_MODE=local and E2E_ENABLE_MCP_LOCAL=1 to run the MCP disabled-auth smoke test.',
);

async function connectMCP(request: APIRequestContext): Promise<MCPClient> {
  let nextId = 1;
  const mcpHeaders = { Accept: 'application/json, text/event-stream' };
  const initialize = await request.post('/mcp', {
    headers: mcpHeaders,
    data: {
      jsonrpc: '2.0',
      id: nextId++,
      method: 'initialize',
      params: {
        protocolVersion: '2025-06-18',
        capabilities: {},
        clientInfo: { name: 'leafwiki-e2e', version: 'test' },
      },
    },
  });
  expect(initialize.ok()).toBeTruthy();

  const sessionId = initialize.headers()['mcp-session-id'];
  expect(sessionId).toBeTruthy();

  const initialized = await request.post('/mcp', {
    headers: { ...mcpHeaders, 'Mcp-Session-Id': sessionId },
    data: {
      jsonrpc: '2.0',
      method: 'notifications/initialized',
      params: {},
    },
  });
  expect(initialized.ok()).toBeTruthy();

  return {
    async callTool(name: string, args: Record<string, unknown> = {}) {
      const response = await request.post('/mcp', {
        headers: { ...mcpHeaders, 'Mcp-Session-Id': sessionId },
        data: {
          jsonrpc: '2.0',
          id: nextId++,
          method: 'tools/call',
          params: {
            name,
            arguments: args,
          },
        },
      });
      expect(response.ok()).toBeTruthy();
      const body = await response.json();
      expect(body.error).toBeFalsy();
      expect(body.result?.isError).toBeFalsy();
      return body.result.structuredContent as Record<string, unknown>;
    },
  };
}

test('mcp disable auth seeds page and UI edit is readable through mcp', async ({
  page,
  request,
}) => {
  const mcp = await connectMCP(request);
  const slug = `mcp-e2e-${Date.now()}`;
  const title = 'MCP E2E Page';

  const created = await mcp.callTool('create_page', { title, slug, kind: 'page' });
  const createdPage = created.page as { id: string; version: string };

  await mcp.callTool('update_page', {
    id: createdPage.id,
    version: createdPage.version,
    title,
    slug,
    content: 'Seeded through MCP',
  });

  const viewPage = new ViewPage(page);
  await viewPage.goto(`/${slug}`);
  await expect(page.locator('article')).toContainText('Seeded through MCP');

  await viewPage.clickEditPageButton();
  const editPage = new EditPage(page);
  await editPage.writeContent('\nUpdated from the UI');
  await editPage.savePage();
  await editPage.closeEditor();

  const readBack = await mcp.callTool('get_page', { id: createdPage.id });
  const pageFromMCP = readBack.page as { content: string };
  expect(pageFromMCP.content).toContain('Updated from the UI');
});
