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

// newEnsurePathTool registers the ensure_path tool: it ensures a route path
// exists, creating the missing segments (intermediate segments become
// sections, the last takes the given kind) and reusing every existing node
// untouched, and returns the final node in the get_page shape. The role gate
// mirrors the REST ensure route, which requires editor.
func newEnsurePathTool(log *slog.Logger, ops PageOps) mcpserver.ServerTool {
	tool := mcpgo.NewTool("ensure_path",
		mcpgo.WithDescription("Ensure a route path (e.g. 'docs/intro') exists, creating the missing segments: intermediate segments become sections, the last segment becomes a page (or a section with kind 'section'). Existing nodes are reused; an intermediate page is converted to a section to hold its children (as the REST create/ensure routes do); a repeated call creates nothing and leaves titles untouched. Returns the final node with line numbers. Requires editor or admin role."),
		mcpgo.WithString("path",
			mcpgo.Required(),
			mcpgo.Description("The route path to ensure"),
		),
		mcpgo.WithString("title",
			mcpgo.Required(),
			mcpgo.Description("The title of the last segment (an existing node keeps its own title)"),
		),
		mcpgo.WithString("kind",
			mcpgo.Description("The kind of the last segment: 'page' (default) or 'section'"),
		),
	)
	return mcpserver.ServerTool{
		Tool: tool,
		Handler: func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			if toolErr := requireRole(ctx, coreauth.RoleEditor); toolErr != nil {
				return toolResult(log, tool.Name, toolErr), nil
			}
			// A leading/trailing slash or whitespace is a client
			// convenience, trimmed the way move_page trims its path.
			path := strings.Trim(req.GetString("path", ""), " /")
			title := strings.TrimSpace(req.GetString("title", ""))
			kind := strings.TrimSpace(req.GetString("kind", ""))
			if message := validateEnsurePathArgs(path, title, kind); message != "" {
				return toolResult(log, tool.Name, &ToolError{Code: ErrCodeMCPInvalidInput, Message: message}), nil
			}
			page, err := ops.EnsurePath(ctx, EnsurePathInput{
				Actor: actorFromContext(ctx), Path: path, Title: title, Kind: kind,
			})
			if err != nil {
				return toolResult(log, tool.Name, toolErrorFrom(err)), nil
			}
			return mcpgo.NewToolResultText(renderPage(page)), nil
		},
	}
}

// validateEnsurePathArgs bounds the arguments and checks the kind value,
// returning the client-facing message or "" when the arguments are usable.
func validateEnsurePathArgs(path, title, kind string) string {
	switch {
	case path == "":
		return "path is required"
	case len(path) > maxRoutePathArgLength:
		return fmt.Sprintf("path is too long (maximum %d characters)", maxRoutePathArgLength)
	case strings.Contains(path, "//"):
		return "path must not contain empty segments"
	case title == "":
		return "title is required"
	case len(title) > maxPageTitleArgLength:
		return fmt.Sprintf("title is too long (maximum %d characters)", maxPageTitleArgLength)
	case kind != "" && kind != "page" && kind != "section":
		return "kind must be 'page' or 'section'"
	}
	return ""
}
