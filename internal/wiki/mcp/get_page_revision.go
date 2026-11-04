package mcp

import (
	"context"
	"log/slog"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	coreauth "github.com/perber/wiki/internal/core/auth"
)

// newGetPageRevisionTool registers the get_page_revision tool: it reads one
// revision of a page — its metadata, the content with line numbers and the
// assets of that point in time. Gated at viewer, the minimum role the REST
// revision read routes (the plain API auth group) require.
func newGetPageRevisionTool(log *slog.Logger, ops PageOps) mcpserver.ServerTool {
	tool := mcpgo.NewTool("get_page_revision",
		mcpgo.WithDescription("Read one revision of a page: its metadata, the content with line numbers and the assets as of that revision. Use a revision ID from get_page_revisions. Requires viewer or admin role."),
		mcpgo.WithString("id",
			mcpgo.Required(),
			mcpgo.Description("The stable page ID"),
		),
		mcpgo.WithString("revision_id",
			mcpgo.Required(),
			mcpgo.Description("The revision ID from get_page_revisions"),
		),
	)
	return mcpserver.ServerTool{
		Tool: tool,
		Handler: func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			if toolErr := requireRole(ctx, coreauth.RoleViewer); toolErr != nil {
				return toolResult(log, tool.Name, toolErr), nil
			}
			id, message := pageIDArg(req)
			revisionID := ""
			if message == "" {
				revisionID, message = revisionIDArg("revision_id", req.GetString("revision_id", ""), true)
			}
			if message != "" {
				return toolResult(log, tool.Name, &ToolError{Code: ErrCodeMCPInvalidInput, Message: message}), nil
			}
			snap, err := ops.GetRevision(ctx, GetRevisionInput{ID: id, RevisionID: revisionID})
			if err != nil {
				return toolResult(log, tool.Name, toolErrorFrom(err)), nil
			}
			return mcpgo.NewToolResultText(renderRevision(snap)), nil
		},
	}
}
