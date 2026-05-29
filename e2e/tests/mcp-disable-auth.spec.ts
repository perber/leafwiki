import { expect, test } from '@playwright/test';
import EditPage from '../pages/EditPage';
import ViewPage from '../pages/ViewPage';
import { connectMCPClient } from './mcpClient';

test.skip(
  process.env.E2E_RUN_MODE !== 'local' || process.env.E2E_ENABLE_MCP_LOCAL !== '1',
  'Set E2E_RUN_MODE=local and E2E_ENABLE_MCP_LOCAL=1 to run the MCP disabled-auth smoke test.',
);

function appURL(path: string): string {
  return new URL(path, process.env.E2E_BASE_URL || 'http://localhost:8080').toString();
}

test('mcp disable auth seeds page and UI edit is readable through mcp', async ({ page }) => {
  const mcp = await connectMCPClient(appURL('/mcp'));
  const slug = `mcp-e2e-${Date.now()}`;
  const title = 'MCP E2E Page';

  try {
    const tools = await mcp.listTools();
    expect(tools).toContain('create_page');
    expect(tools).toContain('update_page');
    expect(tools).toContain('get_page');

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
  } finally {
    await mcp.close();
  }
});
