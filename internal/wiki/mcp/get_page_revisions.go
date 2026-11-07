package mcp

import (
	"context"
	"fmt"
	"log/slog"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	coreauth "github.com/perber/wiki/internal/core/auth"
)

// The revision list's page size. The default and the hard cap mirror the REST
// list route (50 default, 200 max); a request beyond the cap is clamped, like
// search's limit, instead of answered with an error.
const (
	revisionListDefaultLimit = 50
	revisionListMaxLimit     = 200
)

// newGetPageRevisionsTool registers the get_page_revisions tool: it lists a
// page's revision history, newest first, carrying the IDs that
// get_page_revision and restore_page_revision take. Gated at viewer, the
// minimum role the REST revision read routes (the plain API auth group)
// require.
func newGetPageRevisionsTool(log *slog.Logger, ops PageOps) mcpserver.ServerTool {
	tool := mcpgo.NewTool("get_page_revisions",
		mcpgo.WithDescription("List a page's revision history, newest first. Returns each revision's ID (for get_page_revision and restore_page_revision), type, author ID, creation time and summary. Requires viewer or admin role."),
		mcpgo.WithString("id",
			mcpgo.Required(),
			mcpgo.Description("The stable page ID"),
		),
		mcpgo.WithString("cursor",
			mcpgo.Description("Cursor from a previous call's next cursor; omit to start at the newest revision"),
		),
		mcpgo.WithInteger("limit",
			mcpgo.Description(fmt.Sprintf("Maximum number of revisions (default %d, max %d)", revisionListDefaultLimit, revisionListMaxLimit)),
		),
	)
	return mcpserver.ServerTool{
		Tool: tool,
		Handler: func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			if toolErr := requireRole(ctx, coreauth.RoleViewer); toolErr != nil {
				return toolResult(log, tool.Name, toolErr), nil
			}
			id, message := pageIDArg(req)
			cursor := ""
			if message == "" {
				cursor, message = revisionIDArg("cursor", req.GetString("cursor", ""), false)
			}
			if message != "" {
				return toolResult(log, tool.Name, &ToolError{Code: ErrCodeMCPInvalidInput, Message: message}), nil
			}
			limit := req.GetInt("limit", revisionListDefaultLimit)
			if limit <= 0 {
				limit = revisionListDefaultLimit
			}
			if limit > revisionListMaxLimit {
				limit = revisionListMaxLimit
			}
			res, err := ops.ListRevisions(ctx, ListRevisionsInput{ID: id, Cursor: cursor, Limit: limit})
			if err != nil {
				return toolResult(log, tool.Name, toolErrorFrom(err)), nil
			}
			return mcpgo.NewToolResultText(renderRevisions(id, res)), nil
		},
	}
}
