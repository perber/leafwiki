package mcp

import (
	"context"
	"log/slog"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	coreauth "github.com/perber/wiki/internal/core/auth"
)

// newRestorePageRevisionTool registers the restore_page_revision tool: it
// restores a page to a revision and returns the restored page. The role gate
// mirrors the REST restore route, which sits behind RequireEditorOrAdmin; the
// restore itself runs through the RestoreRevisionUseCase, so the pagesave
// event and the save metrics fire exactly as through HTTP — and the restore
// is recorded as a new revision.
func newRestorePageRevisionTool(log *slog.Logger, ops PageOps) mcpserver.ServerTool {
	tool := mcpgo.NewTool("restore_page_revision",
		mcpgo.WithDescription("Restore a page to a revision and return the restored page. The restore is recorded as a new revision in the page's history. Requires editor or admin role."),
		mcpgo.WithString("id",
			mcpgo.Required(),
			mcpgo.Description("The stable page ID"),
		),
		mcpgo.WithString("revision_id",
			mcpgo.Required(),
			mcpgo.Description("The revision ID from get_page_revisions to restore to"),
		),
	)
	return mcpserver.ServerTool{
		Tool: tool,
		Handler: func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			if toolErr := requireRole(ctx, coreauth.RoleEditor); toolErr != nil {
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
			page, err := ops.RestoreRevision(ctx, RestoreRevisionInput{
				Actor:      actorFromContext(ctx),
				ID:         id,
				RevisionID: revisionID,
			})
			if err != nil {
				return toolResult(log, tool.Name, toolErrorFrom(err)), nil
			}
			return mcpgo.NewToolResultText(renderPage(page)), nil
		},
	}
}
