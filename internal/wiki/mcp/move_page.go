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

// newMovePageTool registers the move_page tool: it moves a page to a new
// route path — the last segment becomes the page's slug, the remaining
// segments name the destination parent (which must exist), and an empty path
// moves the page to the wiki root. The role gate mirrors the REST move route,
// which requires editor.
func newMovePageTool(log *slog.Logger, ops PageOps) mcpserver.ServerTool {
	tool := mcpgo.NewTool("move_page",
		mcpgo.WithDescription("Move a page to a new route path (e.g. 'docs/intro') and confirm the new path. The last path segment becomes the page's slug; the remaining segments name the destination parent, which must exist. Omit the path (or pass an empty string) to move the page to the wiki root. Moving into the page itself or its own subtree is refused before anything changes. A move that fails for a reason only the move itself discovers (a slug collision at the destination) may leave the new slug in place. Requires editor or admin role."),
		mcpgo.WithString("id",
			mcpgo.Required(),
			mcpgo.Description("The stable page ID"),
		),
		mcpgo.WithString("path",
			mcpgo.Description("The page's new route path. Omit for the wiki root"),
		),
	)
	return mcpserver.ServerTool{
		Tool: tool,
		Handler: func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			if toolErr := requireRole(ctx, coreauth.RoleEditor); toolErr != nil {
				return toolResult(log, tool.Name, toolErr), nil
			}
			id, message := pageIDArg(req)
			// A leading/trailing slash or whitespace is a client
			// convenience, trimmed the way get_page_by_path trims its
			// path; the route lookup rejects the rest.
			path := strings.Trim(req.GetString("path", ""), " /")

			if message == "" && len(path) > maxRoutePathArgLength {
				message = fmt.Sprintf("path is too long (maximum %d characters)", maxRoutePathArgLength)
			}
			if message == "" && strings.Contains(path, "//") {
				message = "path must not contain empty segments"
			}
			if message != "" {
				return toolResult(log, tool.Name, &ToolError{Code: ErrCodeMCPInvalidInput, Message: message}), nil
			}
			if err := ops.MovePage(ctx, MovePageInput{Actor: actorFromContext(ctx), ID: id, Path: path}); err != nil {
				return toolResult(log, tool.Name, toolErrorFrom(err)), nil
			}
			if path == "" {
				return mcpgo.NewToolResultText(fmt.Sprintf("Page %s moved to the wiki root.", id)), nil
			}
			return mcpgo.NewToolResultText(fmt.Sprintf("Page %s moved to /%s.", id, path)), nil
		},
	}
}
