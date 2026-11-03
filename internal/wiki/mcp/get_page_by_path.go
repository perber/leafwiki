package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	coreauth "github.com/perber/wiki/internal/core/auth"
)

// maxRoutePathArgLength bounds the path argument. A route path is a short
// slug chain; without the cap an oversized body reaches the 1 MiB transport
// cap and fails as an opaque JSON-RPC parse error instead of a clean
// mcp_invalid_input the client can act on.
const maxRoutePathArgLength = 512

// newGetPageByPathTool registers the get_page_by_path tool: it reads a page by
// its route path (the wiki URL path, not a filesystem path) and returns its
// metadata plus the markdown body with line numbers.
func newGetPageByPathTool(log *slog.Logger, ops PageOps) mcpserver.ServerTool {
	tool := mcpgo.NewTool("get_page_by_path",
		mcpgo.WithDescription("Read a page by its route path (the wiki URL path, not a filesystem path). Returns the title, ID, kind, metadata and the markdown body with line numbers."),
		mcpgo.WithString("path",
			mcpgo.Required(),
			mcpgo.Description("The page's route path, e.g. 'docs/intro'"),
		),
	)
	return mcpserver.ServerTool{
		Tool: tool,
		Handler: func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			// A leading/trailing slash or whitespace is a client
			// convenience (the REST route trims the same way); the
			// route lookup rejects an empty segment, so normalise it
			// away.
			path := strings.Trim(req.GetString("path", ""), " /")
			if toolErr := requireRole(ctx, coreauth.RoleViewer); toolErr != nil {
				return toolResult(log, tool.Name, toolErr), nil
			}
			if path == "" {
				return toolResult(log, tool.Name, &ToolError{Code: ErrCodeMCPInvalidInput, Message: "path is required"}), nil
			}
			if len(path) > maxRoutePathArgLength {
				return toolResult(log, tool.Name, &ToolError{
					Code:    ErrCodeMCPInvalidInput,
					Message: fmt.Sprintf("path is too long (maximum %d characters)", maxRoutePathArgLength),
				}), nil
			}
			page, err := ops.GetPageByPath(ctx, path)
			if err != nil {
				return toolResult(log, tool.Name, toolErrorFrom(err)), nil
			}
			return mcpgo.NewToolResultText(renderPage(page)), nil
		},
	}
}
