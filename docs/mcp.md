# Local MCP Interface

LeafWiki can expose a local-only MCP Streamable HTTP endpoint for agents that need to collaborate with a human using the web UI against the same live wiki state.

MCP is disabled by default. Start authenticated local MCP on a loopback host:

```bash
leafwiki --enable-mcp --host 127.0.0.1 --allow-insecure=true --jwt-secret=<secret> --admin-password=<password>
```

The endpoint is fixed at:

```text
http://127.0.0.1:8080/mcp
```

When `--base-path /wiki` is configured, the endpoint is `http://127.0.0.1:8080/wiki/mcp`.
For plain local HTTP, `--allow-insecure=true` is required so LeafWiki can issue login and OAuth cookies without TLS. In production-style TLS setups, provide the same auth secrets without `--allow-insecure`.

## Security Model

MCP only starts when `--enable-mcp` is set and the server host is loopback-only: `localhost`, `127.0.0.1`, or `::1`. Do not expose this endpoint through Docker port publishing, a public reverse proxy, or a public network. A trusted reverse proxy may front LeafWiki for local/private use, but the MCP endpoint must still remain loopback/private.

In normal authenticated mode, `/mcp` is protected by OAuth bearer tokens. Missing, invalid, expired, or insufficient-scope tokens are rejected before MCP requests reach tools. MCP requests do not use LeafWiki CSRF middleware, and OAuth tokens are separate from LeafWiki web JWT cookies.

OAuth authorization requires a logged-in LeafWiki web user and an explicit browser approval step before an authorization code is issued. This prevents another local process from silently minting MCP tokens by opening the user's browser to a loopback callback. PKCE is still required, but PKCE only protects the code exchange for the requesting client; it is not treated as proof of user intent.

The OAuth token identifies a LeafWiki user. LeafWiki loads the current user on every MCP request, so deleting the user or changing the user role affects existing tokens immediately. Read tools are allowed for any authenticated user; mutation tools require the current role to be `editor` or `admin`.

Logging out of the LeafWiki web UI clears the browser session and CSRF cookies; it does not revoke already-issued MCP OAuth access or refresh tokens. In this MVP, token-backed MCP access ends when the user is removed, the access token expires, the refresh token expires, or the server restarts. Role changes take effect immediately for tool permissions: read tools remain available to authenticated users, while mutation tools require the current role to be `editor` or `admin`.

Legacy disabled-auth mode is still available for isolated local workflows:

```bash
leafwiki --disable-auth --enable-mcp --host 127.0.0.1
```

In legacy mode, MCP uses the same effective actor as the disabled-auth UI: `public-editor` with the `editor` role.

## OAuth Client Settings

OAuth-capable MCP clients should use OAuth discovery and Dynamic Client Registration. LeafWiki keeps the fixed `leafwiki-local-mcp` public client for manual testing and backward compatibility, but normal clients should register their own loopback public client before authorization.

- MCP endpoint: `<origin><basePath>/mcp`
- Client ID: dynamically registered; manual/testing fallback `leafwiki-local-mcp`
- Client authentication: public client, no secret
- Scope: `leafwiki:mcp`
- DCR grant types: absent defaults to `authorization_code` and `refresh_token`; clients that explicitly register only `authorization_code` do not receive refresh tokens
- PKCE: required, `S256`
- Authorization endpoint: `<origin><basePath>/oauth/authorize`
- Token endpoint: `<origin><basePath>/oauth/token`
- Dynamic client registration endpoint: `<origin><basePath>/oauth/register`

LeafWiki also publishes OAuth discovery metadata:

- `/.well-known/oauth-protected-resource`
- `/.well-known/oauth-protected-resource/mcp`
- `/.well-known/oauth-protected-resource/<base-path>/mcp`
- `/.well-known/oauth-authorization-server`
- `/.well-known/oauth-authorization-server/<base-path>`

OAuth tokens use in-memory server storage in this MVP. OAuth-capable MCP clients should handle refresh tokens normally, but server restart requires clients to re-authorize. There is no revocation endpoint.

When reverse-proxy remote-user authentication is enabled on a trusted local/private deployment, trusted remote-user requests can use the same OAuth authorize endpoint without a LeafWiki password login. The browser still shows the local approval screen, and the issued MCP token is bound to the resolved LeafWiki user from the trusted header.

## Collaboration Model

The web UI and MCP server run in the same LeafWiki process and share the same `*wiki.Wiki` instance. Pages, search indexes, link indexes, tags, properties, assets, revisions, and refactor operations are updated through the same domain use cases used by the HTTP API.

This means an agent can create or update a page through MCP and a human can immediately see it in the UI. A human can edit through the UI and an agent can read the updated page through MCP.

## Tools

Tool names are intentionally unprefixed.

Always available:

- `get_config`
- `get_current_user`
- `get_tree`
- `get_page`
- `get_page_by_path`
- `lookup_path`
- `resolve_permalink`
- `suggest_slug`
- `create_page`
- `update_page`
- `delete_page`
- `move_page`
- `sort_pages`
- `ensure_page`
- `convert_page`
- `copy_page`
- `search_pages`
- `get_search_status`
- `list_tags`
- `get_pages_by_tags`
- `list_property_keys`
- `get_pages_by_property`
- `get_link_status`
- `upload_asset`
- `get_asset`
- `list_assets`
- `rename_asset`
- `delete_asset`

`get_page` and `get_page_by_path` return `{ page, linkStatus }`. The `linkStatus` field uses the same shape as `get_link_status.status` and includes backlinks, broken incoming links, outgoing links, broken outgoing links, and counts so page reads carry the document context shown in the web UI.

Only available with `--enable-revision`:

- `list_revisions`
- `get_latest_revision`
- `get_revision`
- `compare_revisions`
- `get_revision_asset`
- `restore_revision`

Only available with `--enable-link-refactor`:

- `preview_page_refactor`
- `apply_page_refactor`

## Unsupported Operations

MCP intentionally does not expose importer operations, branding operations or branding resources, login, refresh token, logout, password change, user administration, or admin-only settings.

## Pagination

`search_pages` uses LeafWiki's HTTP search contract with `offset` and `limit`, and returns `count`, `items`, `limit`, `offset`, `tagFacets`, and `hasMore`.

MCP protocol pagination is only for MCP feature lists such as `ListTools`. LeafWiki sets a deterministic tool-list page size in tests so SDK cursor behavior is covered.

## Assets

Asset uploads use:

```json
{ "pageId": "...", "filename": "note.txt", "contentBase64": "..." }
```

Asset reads return:

```json
{ "filename": "note.txt", "mimeType": "text/plain; charset=utf-8", "contentBase64": "..." }
```

## Verification Contract

The local MCP surface is complete only when every defined MCP tool has HTTP/MCP parity coverage or is correctly absent when gated, OAuth coverage passes, and the full project verification passes:

```bash
go test ./...
npm --prefix ui/leafwiki-ui run build
npm --prefix ui/leafwiki-ui run lint
npm --prefix e2e run lint
npm --prefix e2e run format:check
env E2E_RUN_MODE=local E2E_ENABLE_MCP_OAUTH_LOCAL=1 ./e2e/run.sh --grep "mcp.*oauth|oauth.*mcp"
env E2E_RUN_MODE=local E2E_ENABLE_MCP_LOCAL=1 ./e2e/run.sh --grep "mcp.*disable auth|disable auth.*mcp"
```

Normal local E2E mode remains authenticated. Set `E2E_ENABLE_MCP_OAUTH_LOCAL=1` for authenticated MCP OAuth smoke coverage, or `E2E_ENABLE_MCP_LOCAL=1` for the legacy disabled-auth MCP smoke test.

Implementation references:

- Authenticated MCP OAuth follow-up: `codex://threads/019e6e13-1e91-7070-be89-2a45203ea1f6`
- Original local MCP implementation: `codex://threads/019e68f2-8f73-70f0-949b-97271752d87c`
