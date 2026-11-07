package mcp

import (
	"context"
	"log/slog"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	coreauth "github.com/perber/wiki/internal/core/auth"
)

// newComparePageRevisionsTool registers the compare_page_revisions tool: it
// compares two revisions of one page — whether the content differs, the asset
// differences, and both contents with line numbers. The comparison shape is
// the REST compare route's; no diff is computed here. Gated at viewer, the
// minimum role the REST revision read routes require.
func newComparePageRevisionsTool(log *slog.Logger, ops PageOps) mcpserver.ServerTool {
	tool := mcpgo.NewTool("compare_page_revisions",
		mcpgo.WithDescription("Compare two revisions of a page: the content-change flag, the asset differences and both contents with line numbers. Use revision IDs from get_page_revisions. Requires viewer or admin role."),
		mcpgo.WithString("id",
			mcpgo.Required(),
			mcpgo.Description("The stable page ID"),
		),
		mcpgo.WithString("base_revision_id",
			mcpgo.Required(),
			mcpgo.Description("The base revision ID from get_page_revisions"),
		),
		mcpgo.WithString("target_revision_id",
			mcpgo.Required(),
			mcpgo.Description("The target revision ID from get_page_revisions"),
		),
	)
	return mcpserver.ServerTool{
		Tool: tool,
		Handler: func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			if toolErr := requireRole(ctx, coreauth.RoleViewer); toolErr != nil {
				return toolResult(log, tool.Name, toolErr), nil
			}
			id, message := pageIDArg(req)
			baseID, targetID := "", ""
			if message == "" {
				baseID, message = revisionIDArg("base_revision_id", req.GetString("base_revision_id", ""), true)
			}
			if message == "" {
				targetID, message = revisionIDArg("target_revision_id", req.GetString("target_revision_id", ""), true)
			}
			if message != "" {
				return toolResult(log, tool.Name, &ToolError{Code: ErrCodeMCPInvalidInput, Message: message}), nil
			}
			cmp, err := ops.CompareRevisions(ctx, CompareRevisionsInput{ID: id, BaseRevisionID: baseID, TargetRevisionID: targetID})
			if err != nil {
				return toolResult(log, tool.Name, toolErrorFrom(err)), nil
			}
			return mcpgo.NewToolResultText(renderComparison(cmp)), nil
		},
	}
}
