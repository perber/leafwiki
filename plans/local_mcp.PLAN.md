# Local MCP Streamable HTTP Interface For LeafWiki

## Summary

Implement a local-only MCP interface for LeafWiki using the official Go MCP SDK and Streamable HTTP transport. MCP must run in the same process and share the same `*wiki.Wiki` instance as the UI, so humans using the web UI and agents using MCP collaborate against one live wiki state.

Implementation reference thread: `codex://threads/019e68f2-8f73-70f0-949b-97271752d87c`

Hard product contract:

- MCP is disabled by default.
- MCP can only start when `--enable-mcp` and `--disable-auth` are both set.
- MCP must reject non-loopback hosts at startup.
- MCP endpoint is fixed at `<base-path>/mcp`, for example `/mcp` or `/wiki/mcp`.
- MCP mirrors web-server functionality available in `--disable-auth` mode only.
- Importer and branding are intentionally not part of this MCP surface.
- Definition of Done: every defined MCP tool has passing HTTP/MCP parity coverage, and the full test suite is passing.

## Key Changes

- Add dependency on `github.com/modelcontextprotocol/go-sdk/mcp`, pinned to the same commit as `references/go-sdk` (`2d47cc96646020b446391d0b039e1ea5ec06414f`). Use normal Go module resolution; do not add a `replace` to `./references/go-sdk`.
- Add CLI/env config:
  - `--enable-mcp`, default `false`
  - `LEAFWIKI_ENABLE_MCP`
  - `http.RouterOptions.MCPEnabled bool`
- Add startup validation:
  - fail if `enableMCP && !disableAuth`
  - fail if `enableMCP` and host is not `localhost`, `127.0.0.1`, or `::1`
- Add `internal/wiki/mcp` as a normal route registrar. It should mount `GET`, `POST`, and `DELETE` at `<base-path>/mcp` using `mcp.NewStreamableHTTPHandler`.
- Use SDK options:
  - `Stateless: false`
  - `JSONResponse: true`
  - `SessionTimeout: 30 * time.Minute`
  - `DisableLocalhostProtection: false`
  - set `ServerOptions.PageSize` to a small deterministic value in tests and a reasonable default in production so MCP feature-list pagination is exercised.
- Do not attach LeafWiki auth or CSRF middleware to `/mcp`; the safety gate is startup-only local disabled-auth mode.
- MCP mutations must use the same effective actor as disable-auth UI: `public-editor`, role `editor`. Do not use admin or system identity.

## MCP Surface

Expose tools for editor/public operations available through the web server in `--disable-auth` mode, excluding importer and branding. Tool names must not use a `leafwiki_` prefix.

- Config/session:
  - `get_config`
  - `get_current_user`
- Pages/tree:
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
- Search/tags/properties/links:
  - `search_pages`
  - `get_search_status`
  - `list_tags`
  - `get_pages_by_tags`
  - `list_property_keys`
  - `get_pages_by_property`
  - `get_link_status`
- Assets:
  - `upload_asset`
  - `get_asset`
  - `list_assets`
  - `rename_asset`
  - `delete_asset`

Feature-gated tools:

- Only register revision tools when `--enable-revision` is true:
  - `list_revisions`
  - `get_latest_revision`
  - `get_revision`
  - `compare_revisions`
  - `get_revision_asset`
  - `restore_revision`
- Only register link-refactor tools when `--enable-link-refactor` is true:
  - `preview_page_refactor`
  - `apply_page_refactor`

Do not expose:

- importer tools
- branding tools or branding resources
- login, refresh token, logout, password change
- create/list/update/delete users
- admin-only settings

Implementation rules:

- Use existing domain use cases, not direct store mutation and not internal HTTP proxying.
- Preserve optimistic version behavior for update/delete/move/convert/refactor apply.
- Preserve metadata behavior: page update with `tags` and `properties` must build the same markdown frontmatter as the HTTP route.
- For asset binary inputs, use `{filename, contentBase64}`. For asset reads, return `{filename, mimeType, contentBase64}`.
- UI search is paginated with `offset` and `limit`; `search_pages` must expose the same fields and return `{count, items, limit, offset, tagFacets, hasMore}`.
- MCP protocol pagination applies to MCP feature lists such as tools/resources/prompts, not arbitrary tool outputs. Use SDK pagination for tool/resource listing; use LeafWiki’s existing offset/limit contract for `search_pages`.

## Test Plan

Add a comprehensive Go-first parity suite.

- SDK/registration tests:
  - MCP disabled by default: `/mcp` is unavailable.
  - MCP route appears at `/mcp` and `<base-path>/mcp` only when enabled.
  - tool list matches enabled feature flags.
  - tool names have no `leafwiki_` prefix.
  - importer, branding, auth, and admin tools are absent.
  - SDK tool listing paginates via MCP `ListTools` cursor behavior when test `PageSize` is smaller than the tool count.
- CLI validation tests:
  - `--enable-mcp` without `--disable-auth` fails.
  - `--enable-mcp --disable-auth --host=0.0.0.0` fails.
  - loopback hosts pass.
- Protocol integration tests:
  - start one `wiki.Wiki` with `AuthDisabled: true`
  - build one router with HTTP API and MCP mounted
  - connect using `mcp.NewClient` and `mcp.StreamableClientTransport`
  - verify `ListTools`, paginated `ListTools`, search, get page, create page, update page
- HTTP/MCP parity tests:
  - every defined non-feature-gated tool has a parity test against the equivalent HTTP route.
  - every defined feature-gated tool has a parity test when its feature flag is enabled.
  - HTTP creates page, MCP reads/searches it.
  - MCP creates/updates/moves/deletes page, HTTP sees the result.
  - stale version conflicts behave the same through both surfaces.
  - paginated search returns the same `count`, `offset`, `limit`, and items as HTTP search.
  - tags/properties/search/link indexes update after MCP writes.
  - assets upload/list/read/rename/delete match HTTP behavior.
  - revisions record HTTP and MCP writes when revisions are enabled.
  - revision/link-refactor tools are absent when flags are disabled.
  - blocked/excluded operations stay unavailable through MCP.
- Playwright smoke tests:
  - add local E2E mode that starts `--disable-auth --enable-mcp --enable-revision --enable-link-refactor`
  - seed a page through MCP, verify UI renders it
  - update via UI, verify MCP reads the updated page

Definition of Done verification:

```bash
go test ./...
go test ./cmd/leafwiki ./internal/http ./internal/wiki/... ./internal/...
npm --prefix ui/leafwiki-ui run build
E2E_RUN_MODE=local ./e2e/run.sh --grep "mcp|disable auth"
```

The plan is not complete until all commands pass and the parity suite proves every defined MCP tool either matches its HTTP equivalent or is correctly absent when gated/unsupported.

## Documentation And References

- Add `docs/mcp.md` covering:
  - local-only purpose
  - example command: `leafwiki --disable-auth --enable-mcp --host 127.0.0.1`
  - endpoint: `http://127.0.0.1:8080/mcp`
  - agent + UI collaboration model
  - disabled-auth requirement
  - loopback-only requirement
  - exposed and intentionally unsupported operations
  - unprefixed tool names
  - search pagination via `offset` and `limit`
  - MCP protocol pagination for feature lists
  - Definition of Done parity requirement
- Update `readme.md`:
  - CLI flags table with `--enable-mcp`
  - environment variable table with `LEAFWIKI_ENABLE_MCP`
  - local MCP usage section that links to `docs/mcp.md`
  - do not add Docker or public deployment examples for MCP
- Add source comments only where needed to explain security gates.
- Include implementation reference in docs or PR text:
  - `codex://threads/019e68f2-8f73-70f0-949b-97271752d87c`
- Use these SDK references during implementation:
  - `references/go-sdk/docs/quick_start.md`
  - `references/go-sdk/docs/server.md`
  - `references/go-sdk/mcp/streamable.go`
  - `references/go-sdk/examples/http/main.go`
