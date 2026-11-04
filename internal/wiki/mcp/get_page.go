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

// maxPageIDArgLength bounds the id argument. A page ID is a short stable
// string; without the cap an oversized body reaches the 1 MiB transport cap
// and fails as an opaque JSON-RPC parse error instead of a clean
// mcp_invalid_input the client can act on.
const maxPageIDArgLength = 128

// newGetPageTool registers the get_page tool: it reads a page by its stable ID
// and returns its identifying metadata plus the markdown body with line
// numbers. The optional offset/lines pair reads a slice of the body
// (renderPageWindow) instead of the whole page.
func newGetPageTool(log *slog.Logger, ops PageOps) mcpserver.ServerTool {
	tool := mcpgo.NewTool("get_page",
		mcpgo.WithDescription("Read a page by its stable ID. Returns the title, route path, kind, metadata and the markdown body with line numbers. Use offset/lines to read a slice of the page."),
		mcpgo.WithString("id",
			mcpgo.Required(),
			mcpgo.Description("The stable page ID"),
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
			id := strings.TrimSpace(req.GetString("id", ""))
			if toolErr := requireRole(ctx, coreauth.RoleViewer); toolErr != nil {
				return toolResult(log, tool.Name, toolErr), nil
			}
			if id == "" {
				return toolResult(log, tool.Name, &ToolError{Code: ErrCodeMCPInvalidInput, Message: "id is required"}), nil
			}
			if len(id) > maxPageIDArgLength {
				return toolResult(log, tool.Name, &ToolError{
					Code:    ErrCodeMCPInvalidInput,
					Message: fmt.Sprintf("id is too long (maximum %d characters)", maxPageIDArgLength),
				}), nil
			}
			offset, lines, message := pageWindowArgs(req)
			if message != "" {
				return toolResult(log, tool.Name, &ToolError{Code: ErrCodeMCPInvalidInput, Message: message}), nil
			}
			page, err := ops.GetPage(ctx, id)
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
