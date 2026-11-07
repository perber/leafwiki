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

// newConvertNodeTool registers the convert_node tool: it converts a page to
// a section, or a section to a page — the REST convert route's operation,
// with the version a tool caller does not track resolved from the live page.
// A section that still has children cannot become a page (the REST-parity
// page_has_children; the node store's not-empty-folder refusal surfaces the
// mapper's page_convert_not_allowed arm). The role gate mirrors the REST
// convert route, which requires editor.
func newConvertNodeTool(log *slog.Logger, ops PageOps) mcpserver.ServerTool {
	tool := mcpgo.NewTool("convert_node",
		mcpgo.WithDescription("Convert a page to a section, or a section to a page. Converting a section that still has children to a page is refused (page_has_children). Requires editor or admin role."),
		mcpgo.WithString("id",
			mcpgo.Required(),
			mcpgo.Description("The stable page ID"),
		),
		mcpgo.WithString("kind",
			mcpgo.Required(),
			mcpgo.Description("The target kind: 'page' or 'section'"),
		),
	)
	return mcpserver.ServerTool{
		Tool: tool,
		Handler: func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			if toolErr := requireRole(ctx, coreauth.RoleEditor); toolErr != nil {
				return toolResult(log, tool.Name, toolErr), nil
			}
			id, message := pageIDArg(req)
			kind := strings.TrimSpace(req.GetString("kind", ""))
			if message == "" {
				switch {
				case kind == "":
					message = "kind is required"
				case kind != "page" && kind != "section":
					message = "kind must be 'page' or 'section'"
				}
			}
			if message != "" {
				return toolResult(log, tool.Name, &ToolError{Code: ErrCodeMCPInvalidInput, Message: message}), nil
			}
			if err := ops.ConvertNode(ctx, ConvertNodeInput{Actor: actorFromContext(ctx), ID: id, Kind: kind}); err != nil {
				return toolResult(log, tool.Name, toolErrorFrom(err)), nil
			}
			return mcpgo.NewToolResultText(fmt.Sprintf("Converted %s to a %s.", id, kind)), nil
		},
	}
}
