import type {
  AuthProvider,
  OAuthClientInformationMixed,
  OAuthClientProvider,
  OAuthTokens,
  Client as SDKClient,
  StreamableHTTPClientTransport as SDKStreamableHTTPTransport,
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
  return wrapMCPClient(client, transport);
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
          return wrapMCPClient(authenticatedClient, authenticatedTransport);
        },
      };
    }
    throw error;
  }

  await transport.terminateSession();
  await client.close();
  throw new Error('Expected MCP SDK OAuth flow to require authorization');
}

function wrapMCPClient(client: SDKClient, transport: SDKStreamableHTTPTransport): MCPTestClient {
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
        await transport.terminateSession();
      } finally {
        await client.close();
      }
    },
  };
}
