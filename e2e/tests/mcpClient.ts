import { spawn } from 'node:child_process';

import type {
  AuthProvider,
  OAuthClientInformationMixed,
  OAuthClientProvider,
  OAuthTokens,
  Client as SDKClient,
} from '@modelcontextprotocol/client';

export type MCPTestClient = {
  listTools: () => Promise<string[]>;
  callTool: (name: string, args?: Record<string, unknown>) => Promise<Record<string, unknown>>;
  close: () => Promise<void>;
};

type ConnectMCPClientOptions = {
  accessToken?: string;
  clientName?: string;
};

type ConnectMCPStdioClientOptions = ConnectMCPClientOptions & {
  captureStderr?: boolean;
};

type MCPStdioRawOptions = ConnectMCPClientOptions & {
  timeoutMs?: number;
};

type MCPStdioRawError = {
  code?: number;
  data?: {
    status?: number;
    [key: string]: unknown;
  };
  message?: string;
};

type MCPStdioRawResponse = {
  error?: MCPStdioRawError;
  id?: unknown;
  jsonrpc?: string;
  result?: unknown;
};

export type MCPStdioRawResult = {
  exitCode: number | null;
  response?: MCPStdioRawResponse;
  responses: MCPStdioRawResponse[];
  signal: NodeJS.Signals | null;
  stderr: string;
  stdout: string;
  stdoutLines: string[];
};

type SDKOAuthFlowOptions = {
  clientName?: string;
  dynamicRegistration?: boolean;
  redirectURI: string;
};

export type MCPTestOAuthFlow = {
  authorizationURL: URL;
  clientID: () => string | undefined;
  finishAuth: (authorizationCode: string) => Promise<MCPTestClient>;
};

async function loadSDKModule(): Promise<typeof import('@modelcontextprotocol/client')> {
  return import('@modelcontextprotocol/client');
}

function stdioCommand(): string {
  const command = process.env.E2E_MCP_STDIO_COMMAND;
  if (!command) {
    throw new Error('E2E_MCP_STDIO_COMMAND must be set for stdio MCP client tests');
  }
  return command;
}

function leafwikiStdioEnv(
  endpoint: string,
  options: ConnectMCPClientOptions,
): Record<string, string> {
  const env: Record<string, string> = {
    LEAFWIKI_MCP_ENDPOINT: endpoint,
  };
  if (options.accessToken) {
    env.LEAFWIKI_MCP_API_KEY = options.accessToken;
  }
  return env;
}

function leafwikiStdioProcessEnv(
  endpoint: string,
  options: ConnectMCPClientOptions,
): NodeJS.ProcessEnv {
  const env: NodeJS.ProcessEnv = {
    ...process.env,
    LEAFWIKI_MCP_ENDPOINT: endpoint,
  };
  if (options.accessToken) {
    env.LEAFWIKI_MCP_API_KEY = options.accessToken;
  } else {
    delete env.LEAFWIKI_MCP_API_KEY;
  }
  return env;
}

export async function connectMCPClient(
  endpoint: string,
  options: ConnectMCPClientOptions = {},
): Promise<MCPTestClient> {
  const { Client, StreamableHTTPClientTransport } = await loadSDKModule();
  const client = new Client({
    name: options.clientName || 'leafwiki-e2e',
    version: 'test',
  });
  const authProvider: AuthProvider | undefined = options.accessToken
    ? { token: async () => options.accessToken }
    : undefined;
  const transport = new StreamableHTTPClientTransport(new URL(endpoint), {
    authProvider,
  });

  await client.connect(transport);
  return wrapMCPClient(client, async () => {
    await transport.terminateSession();
  });
}

export async function connectMCPStdioClient(
  endpoint: string,
  options: ConnectMCPStdioClientOptions = {},
): Promise<MCPTestClient> {
  const { Client, StdioClientTransport } = await loadSDKModule();
  const env = leafwikiStdioEnv(endpoint, options);

  const client = new Client({
    name: options.clientName || 'leafwiki-e2e-stdio',
    version: 'test',
  });
  const transport = new StdioClientTransport({
    command: stdioCommand(),
    cwd: process.env.E2E_REPO_ROOT,
    env,
    stderr: options.captureStderr === false ? 'inherit' : 'pipe',
  });

  await client.connect(transport);
  return wrapMCPClient(client);
}

export async function requestMCPStdioFrame(
  endpoint: string,
  frame: Record<string, unknown>,
  options: MCPStdioRawOptions = {},
): Promise<MCPStdioRawResult> {
  const child = spawn(stdioCommand(), [], {
    cwd: process.env.E2E_REPO_ROOT,
    env: leafwikiStdioProcessEnv(endpoint, options),
    stdio: ['pipe', 'pipe', 'pipe'],
  });
  let stdout = '';
  let stderr = '';
  let timedOut = false;
  const timeoutMs = options.timeoutMs ?? 5000;

  child.stdout.setEncoding('utf8');
  child.stdout.on('data', (chunk: string) => {
    stdout += chunk;
  });
  child.stderr.setEncoding('utf8');
  child.stderr.on('data', (chunk: string) => {
    stderr += chunk;
  });

  const close = new Promise<{ exitCode: number | null; signal: NodeJS.Signals | null }>(
    (resolve, reject) => {
      child.once('error', reject);
      child.once('close', (exitCode, signal) => {
        resolve({ exitCode, signal });
      });
    },
  );
  const timer = setTimeout(() => {
    timedOut = true;
    child.kill('SIGTERM');
  }, timeoutMs);

  child.stdin.end(`${JSON.stringify(frame)}\n`);
  const closed = await close;
  clearTimeout(timer);
  if (timedOut) {
    throw new Error(`leafwiki-mcp-stdio did not exit within ${timeoutMs}ms; stderr=${stderr}`);
  }

  const stdoutLines = stdout
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line !== '');
  const responses = stdoutLines.map((line) => JSON.parse(line) as MCPStdioRawResponse);
  return {
    ...closed,
    response: responses[0],
    responses,
    stderr,
    stdout,
    stdoutLines,
  };
}

export async function startMCPClientSDKOAuthFlow(
  endpoint: string,
  options: SDKOAuthFlowOptions,
): Promise<MCPTestOAuthFlow> {
  const { Client, StreamableHTTPClientTransport, UnauthorizedError } = await loadSDKModule();
  let authorizationURL: URL | undefined;
  let codeVerifier = '';
  let clientInformation: OAuthClientInformationMixed | undefined = options.dynamicRegistration
    ? undefined
    : { client_id: 'leafwiki-local-mcp' };
  let tokens: OAuthTokens | undefined;
  const clientID = () =>
    typeof clientInformation?.client_id === 'string' ? clientInformation.client_id : undefined;
  const authProvider: OAuthClientProvider = {
    redirectUrl: options.redirectURI,
    clientMetadata: {
      client_name: options.clientName || 'leafwiki-e2e-oauth-discovery',
      redirect_uris: [options.redirectURI],
      grant_types: ['authorization_code', 'refresh_token'],
      response_types: ['code'],
      token_endpoint_auth_method: 'none',
    },
    clientInformation: () => clientInformation,
    saveClientInformation: (nextClientInformation) => {
      clientInformation = nextClientInformation;
    },
    tokens: () => tokens,
    saveTokens: (nextTokens) => {
      tokens = nextTokens;
    },
    redirectToAuthorization: (nextAuthorizationURL) => {
      authorizationURL = nextAuthorizationURL;
    },
    saveCodeVerifier: (nextCodeVerifier) => {
      codeVerifier = nextCodeVerifier;
    },
    codeVerifier: () => codeVerifier,
  };
  const client = new Client({
    name: options.clientName || 'leafwiki-e2e-oauth-discovery',
    version: 'test',
  });
  const transport = new StreamableHTTPClientTransport(new URL(endpoint), { authProvider });

  try {
    await client.connect(transport);
  } catch (error) {
    if (error instanceof UnauthorizedError && authorizationURL) {
      return {
        authorizationURL,
        clientID,
        async finishAuth(authorizationCode: string) {
          await transport.finishAuth(authorizationCode);
          await client.close();
          const authenticatedClient = new Client({
            name: options.clientName || 'leafwiki-e2e-oauth-discovery',
            version: 'test',
          });
          const authenticatedTransport = new StreamableHTTPClientTransport(new URL(endpoint), {
            authProvider,
          });
          await authenticatedClient.connect(authenticatedTransport);
          return wrapMCPClient(authenticatedClient, async () => {
            await authenticatedTransport.terminateSession();
          });
        },
      };
    }
    throw error;
  }

  await transport.terminateSession();
  await client.close();
  throw new Error('Expected MCP SDK OAuth flow to require authorization');
}

function wrapMCPClient(client: SDKClient, closeTransport?: () => Promise<void>): MCPTestClient {
  return {
    async listTools() {
      const result = await client.listTools();
      return result.tools.map((tool) => tool.name);
    },

    async callTool(name: string, args: Record<string, unknown> = {}) {
      const result = await client.callTool({ name, arguments: args });
      if ('isError' in result && result.isError) {
        throw new Error(`MCP tool ${name} returned an error: ${JSON.stringify(result.content)}`);
      }
      if (!('structuredContent' in result) || !result.structuredContent) {
        throw new Error(`MCP tool ${name} did not return structured content`);
      }
      return result.structuredContent as Record<string, unknown>;
    },

    async close() {
      try {
        await closeTransport?.();
      } finally {
        await client.close();
      }
    },
  };
}
