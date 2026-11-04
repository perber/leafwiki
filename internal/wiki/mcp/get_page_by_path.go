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

// newGetPageByPathTool registers the get_page_by_path tool: it reads a page
// by its route path (the wiki URL path, not a filesystem path) and returns
// its metadata plus the markdown body with line numbers. The optional
// offset/lines pair reads a slice of the body (renderPageWindow) instead of
// the whole page.
func newGetPageByPathTool(log *slog.Logger, ops PageOps) mcpserver.ServerTool {
	tool := mcpgo.NewTool("get_page_by_path",
		mcpgo.WithDescription("Read a page by its route path (the wiki URL path, not a filesystem path). Returns the title, ID, kind, metadata and the markdown body with line numbers. Use offset/lines to read a slice of the page."),
		mcpgo.WithString("path",
			mcpgo.Required(),
			mcpgo.Description("The page's route path, e.g. 'docs/intro'"),
		),
		mcpgo.WithInteger("offset",
			mcpgo.Description("Zero-based line offset to start reading from (default 0)"),
		),
		mcpgo.WithInteger("lines",
			mcpgo.Description("Number of lines to return (default 0 = all remaining)"),
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
			offset, lines, message := pageWindowArgs(req)
			if message != "" {
				return toolResult(log, tool.Name, &ToolError{Code: ErrCodeMCPInvalidInput, Message: message}), nil
			}
			page, err := ops.GetPageByPath(ctx, path)
			if err != nil {
				return toolResult(log, tool.Name, toolErrorFrom(err)), nil
			}
			// An offset past the end of the page is the tool's own
			// mcp_invalid_input (the page exists; the window does not).
			text, err := renderPageWindow(page, offset, lines)
			if err != nil {
				return toolResult(log, tool.Name, &ToolError{Code: ErrCodeMCPInvalidInput, Message: err.Error()}), nil
			}
			return mcpgo.NewToolResultText(text), nil
		},
	}
}
