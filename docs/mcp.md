# Local MCP Interface

LeafWiki can expose a local-only MCP Streamable HTTP endpoint for agents that need to collaborate with a human using the web UI against the same live wiki state.

MCP is disabled by default. Start it only on a loopback host with authentication disabled:

```bash
leafwiki --disable-auth --enable-mcp --host 127.0.0.1
```

The endpoint is fixed at:

```text
http://127.0.0.1:8080/mcp
```

When `--base-path /wiki` is configured, the endpoint is `http://127.0.0.1:8080/wiki/mcp`.

## Security Model

MCP only starts when both `--enable-mcp` and `--disable-auth` are set. Startup fails if the host is not `localhost`, `127.0.0.1`, or `::1`.

The MCP route does not use LeafWiki auth or CSRF middleware. The safety boundary is the startup gate: local loopback plus disabled-auth mode. MCP mutations use the same effective actor as the disabled-auth UI: `public-editor` with the `editor` role.

Do not expose this endpoint through Docker, a reverse proxy, or a public network.

## Collaboration Model

The web UI and MCP server run in the same LeafWiki process and share the same `*wiki.Wiki` instance. Pages, search indexes, link indexes, tags, properties, assets, revisions, and refactor operations are updated through the same domain use cases used by the HTTP API.

This means an agent can create or update a page through MCP and a human can immediately see it in the UI. A human can edit through the UI and an agent can read the updated page through MCP.

## Tools

Tool names are intentionally unprefixed.

Always available in MCP local disabled-auth mode:

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

The local MCP surface is complete only when every defined MCP tool has HTTP/MCP parity coverage or is correctly absent when gated, and the full project verification passes:

```bash
go test ./...
go test ./cmd/leafwiki ./internal/http ./internal/wiki/... ./internal/...
npm --prefix ui/leafwiki-ui run build
E2E_RUN_MODE=local E2E_ENABLE_MCP_LOCAL=1 ./e2e/run.sh --grep "mcp|disable auth"
```

Normal local E2E mode remains authenticated. Set `E2E_ENABLE_MCP_LOCAL=1` only for the disabled-auth MCP smoke test.

Implementation reference: `codex://threads/019e68f2-8f73-70f0-949b-97271752d87c`.
