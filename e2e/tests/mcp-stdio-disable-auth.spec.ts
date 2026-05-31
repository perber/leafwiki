import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { expect, test } from '@playwright/test';
import EditPage from '../pages/EditPage';
import ViewPage from '../pages/ViewPage';
import { toAppPath } from '../pages/appPath';
import { connectMCPStdioClient, requestMCPStdioFrame } from './mcpClient';

test.skip(
  process.env.E2E_RUN_MODE !== 'local' ||
    process.env.E2E_ENABLE_MCP_LOCAL !== '1' ||
    process.env.E2E_MCP_CLIENT_TRANSPORT !== 'stdio',
  'Set E2E_RUN_MODE=local, E2E_ENABLE_MCP_LOCAL=1, and E2E_MCP_CLIENT_TRANSPORT=stdio to run the MCP stdio disabled-auth smoke test.',
);

const assertRootFiles = process.env.E2E_ASSERT_SEPARATE_ROOT_FILES === '1';
const dataDir = process.env.E2E_DATA_DIR ?? '';
const rootDir = process.env.E2E_ROOT_DIR ?? '';

function appURL(routePath: string): string {
  return new URL(
    toAppPath(routePath),
    process.env.E2E_BASE_URL || 'http://localhost:8080',
  ).toString();
}

function expectMarkdownInConfiguredRoot(slug: string, expectedContent: string) {
  expect(dataDir, 'E2E_DATA_DIR should be exported by the local E2E runner').not.toBe('');
  expect(rootDir, 'E2E_ROOT_DIR should be exported by the local E2E runner').not.toBe('');

  const rootFile = path.join(rootDir, `${slug}.md`);
  const defaultRootFile = path.join(dataDir, 'root', `${slug}.md`);

  expect(existsSync(rootFile), `${rootFile} should exist`).toBe(true);
  expect(readFileSync(rootFile, 'utf8')).toContain(expectedContent);
  expect(existsSync(defaultRootFile), `${defaultRootFile} should not exist`).toBe(false);
}

test('mcp stdio sidecar seeds page and UI edit is readable through mcp', async ({ page }) => {
  const mcp = await connectMCPStdioClient(appURL('/mcp'));
  const slug = `mcp-stdio-e2e-${Date.now()}`;
  const title = 'MCP STDIO E2E Page';

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
      content: 'Seeded through MCP STDIO',
    });

    const viewPage = new ViewPage(page);
    await viewPage.goto(`/${slug}`);
    await expect(page.locator('article')).toContainText('Seeded through MCP STDIO');

    if (assertRootFiles) {
      expectMarkdownInConfiguredRoot(slug, 'Seeded through MCP STDIO');
    }

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

test('mcp stdio sidecar raw lifecycle exits after stdin closes', async () => {
  const result = await requestMCPStdioFrame(appURL('/mcp'), {
    jsonrpc: '2.0',
    id: 1,
    method: 'initialize',
    params: {
      clientInfo: { name: 'leafwiki-e2e-stdio-lifecycle', version: 'test' },
      protocolVersion: '2025-11-25',
    },
  });

  expect(result.exitCode).toBe(0);
  expect(result.signal).toBeNull();
  expect(result.stdoutLines).toHaveLength(1);
  expect(result.responses).toHaveLength(1);
  expect(result.response?.result).toBeTruthy();
  expect(result.stderr).not.toContain('shutdown delete failed');
});

test('mcp stdio sidecar uses a base-path endpoint', async () => {
  test.skip(process.env.E2E_BASE_PATH !== '/wiki', 'requires E2E_BASE_PATH=/wiki');

  const mcp = await connectMCPStdioClient(appURL('/mcp'));
  try {
    const config = await mcp.callTool('get_config');
    expect(config.basePath).toBe('/wiki');

    const created = await mcp.callTool('create_page', {
      title: 'MCP STDIO Base Path',
      slug: `mcp-stdio-base-path-${Date.now()}`,
      kind: 'page',
    });
    const createdPage = created.page as { id: string };
    expect(createdPage.id).toBeTruthy();
  } finally {
    await mcp.close();
  }
});
