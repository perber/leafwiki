package mcp

import (
	"context"
	"fmt"
	"log/slog"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	coreauth "github.com/perber/wiki/internal/core/auth"
)

// newDeletePageTool registers the delete_page tool: it deletes a page with the
// REST delete's subtree semantics — without recursive only a childless page
// goes; with recursive the whole subtree goes, and assets, favorites and
// revision data are cleaned up as through HTTP. The role gate mirrors the
// REST delete route, which requires editor.
func newDeletePageTool(log *slog.Logger, ops PageOps) mcpserver.ServerTool {
	tool := mcpgo.NewTool("delete_page",
		mcpgo.WithDescription("Delete a page. Without recursive only a page without children is deleted; with recursive the whole subtree goes, including assets, favorites and revision data. Requires editor or admin role."),
		mcpgo.WithString("id",
			mcpgo.Required(),
			mcpgo.Description("The stable page ID"),
		),
		mcpgo.WithBoolean("recursive",
			mcpgo.Description("Delete the page's subtree as well (default false)"),
		),
	)
	return mcpserver.ServerTool{
		Tool: tool,
		Handler: func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			if toolErr := requireRole(ctx, coreauth.RoleEditor); toolErr != nil {
				return toolResult(log, tool.Name, toolErr), nil
			}
			id, message := pageIDArg(req)
			if message != "" {
				return toolResult(log, tool.Name, &ToolError{Code: ErrCodeMCPInvalidInput, Message: message}), nil
			}
			recursive := req.GetBool("recursive", false)
			if err := ops.DeletePage(ctx, DeletePageInput{Actor: actorFromContext(ctx), ID: id, Recursive: recursive}); err != nil {
				return toolResult(log, tool.Name, toolErrorFrom(err)), nil
			}
			if recursive {
				return mcpgo.NewToolResultText(fmt.Sprintf("Page %s and its subtree deleted.", id)), nil
			}
			return mcpgo.NewToolResultText(fmt.Sprintf("Page %s deleted.", id)), nil
		},
	}
}
