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

// newUpdatePageTool registers the update_page tool: a full replace of the
// page's title, slug and/or content. An omitted field keeps its current value;
// an explicit empty content clears the page. The role gate mirrors the REST
// update route, which requires editor.
func newUpdatePageTool(log *slog.Logger, ops PageOps) mcpserver.ServerTool {
	tool := mcpgo.NewTool("update_page",
		mcpgo.WithDescription("Fully replace a page's title and/or content (omitted fields are kept) and return the updated page with line numbers. An empty content string clears the page. An optional slug renames the page; if omitted, the slug is preserved. Requires editor or admin role."),
		mcpgo.WithString("id",
			mcpgo.Required(),
			mcpgo.Description("The stable page ID"),
		),
		mcpgo.WithString("title",
			mcpgo.Description("The new title. Omit to keep the current title"),
		),
		mcpgo.WithString("content",
			mcpgo.Description("The new markdown body. An empty string clears the page. Omit to keep the current content"),
		),
		mcpgo.WithString("slug",
			mcpgo.Description("New slug (optional). If omitted, slug is preserved."),
		),
	)
	return mcpserver.ServerTool{
		Tool: tool,
		Handler: func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			if toolErr := requireRole(ctx, coreauth.RoleEditor); toolErr != nil {
				return toolResult(log, tool.Name, toolErr), nil
			}
			id, message := pageIDArg(req)
			// Trimmed like create_page's arguments: surrounding whitespace
			// would otherwise be stored as part of the title.
			title := strings.TrimSpace(req.GetString("title", ""))
			args := req.GetArguments()
			content, contentSet := args["content"].(string)
			// Trimmed like create_page's and copy_page's slug arguments:
			// surrounding whitespace would otherwise be stored as part of
			// the slug. An invalid slug is the use case's field error, not
			// the tool's own.
			slug := strings.TrimSpace(req.GetString("slug", ""))
			if message == "" {
				message = validateUpdatePageArgs(title, contentSet, slug)
			}
			if message != "" {
				return toolResult(log, tool.Name, &ToolError{Code: ErrCodeMCPInvalidInput, Message: message}), nil
			}
			in := UpdatePageInput{Actor: actorFromContext(ctx), ID: id, Title: title, Slug: slug}
			if contentSet {
				in.Content = &content
			}
			page, err := ops.UpdatePage(ctx, in)
			if err != nil {
				return toolResult(log, tool.Name, toolErrorFrom(err)), nil
			}
			return mcpgo.NewToolResultText(renderPage(page)), nil
		},
	}
}

// validateUpdatePageArgs bounds the arguments and requires at least one
// change. Content presence (not its value) counts as a change: an empty
// string clears the page, so it is a meaningful update on its own. A slug
// counts as a change too — a slug-only call is a rename, meaningful on its
// own.
func validateUpdatePageArgs(title string, contentSet bool, slug string) string {
	switch {
	case len(title) > maxPageTitleArgLength:
		return fmt.Sprintf("title is too long (maximum %d characters)", maxPageTitleArgLength)
	case title == "" && !contentSet && slug == "":
		return "provide at least one of title, content or slug"
	}
	return ""
}
