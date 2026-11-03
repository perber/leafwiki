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
// and returns its identifying metadata plus the markdown body with line numbers.
func newGetPageTool(log *slog.Logger, ops PageOps) mcpserver.ServerTool {
	tool := mcpgo.NewTool("get_page",
		mcpgo.WithDescription("Read a page by its stable ID. Returns the title, route path, kind, metadata and the markdown body with line numbers."),
		mcpgo.WithString("id",
			mcpgo.Required(),
			mcpgo.Description("The stable page ID"),
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
			page, err := ops.GetPage(ctx, id)
			if err != nil {
				return toolResult(log, tool.Name, toolErrorFrom(err)), nil
			}
			return mcpgo.NewToolResultText(renderPage(page)), nil
		},
	}
}
