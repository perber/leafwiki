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

// newCopyPageTool registers the copy_page tool: it copies a page (and its
// assets) under a new title and slug — the REST copy route's single-page
// semantics, with the children staying with the source — and returns the copy
// in the get_page shape. A destination slug already taken under the same
// parent is refused with the REST-parity page_slug_conflict. The role gate
// mirrors the REST copy route, which requires editor.
func newCopyPageTool(log *slog.Logger, ops PageOps) mcpserver.ServerTool {
	tool := mcpgo.NewTool("copy_page",
		mcpgo.WithDescription("Copy a page (and its assets) under a new title and slug, optionally under a parent section. The copy is always a page, even when the source is a section (REST parity), and copying under a parent page converts that parent to a section to hold the copy (the same create-time behaviour as the REST create/ensure routes). The source is left untouched and its children stay with it, like the REST copy route. A slug already taken under the destination parent is refused (page_slug_conflict). Requires editor or admin role."),
		mcpgo.WithString("id",
			mcpgo.Required(),
			mcpgo.Description("The stable page ID to copy"),
		),
		mcpgo.WithString("title",
			mcpgo.Required(),
			mcpgo.Description("The title of the copy"),
		),
		mcpgo.WithString("slug",
			mcpgo.Required(),
			mcpgo.Description("The slug of the copy"),
		),
		mcpgo.WithString("parentID",
			mcpgo.Description("The ID of the parent section to copy under. Omit to copy at the wiki root"),
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
			// would otherwise be stored as part of the title or slug.
			title := strings.TrimSpace(req.GetString("title", ""))
			slug := strings.TrimSpace(req.GetString("slug", ""))
			parentID := strings.TrimSpace(req.GetString("parentID", ""))
			if message == "" {
				message = validateCopyPageArgs(title, slug, parentID)
			}
			if message != "" {
				return toolResult(log, tool.Name, &ToolError{Code: ErrCodeMCPInvalidInput, Message: message}), nil
			}
			in := CopyPageInput{
				Actor: actorFromContext(ctx), SourceID: id, Title: title, Slug: slug,
			}
			if parentID != "" {
				in.ParentID = &parentID
			}
			page, err := ops.CopyPage(ctx, in)
			if err != nil {
				return toolResult(log, tool.Name, toolErrorFrom(err)), nil
			}
			return mcpgo.NewToolResultText(renderPage(page)), nil
		},
	}
}

// validateCopyPageArgs bounds the short arguments, returning the
// client-facing message or "" when the arguments are usable.
func validateCopyPageArgs(title, slug, parentID string) string {
	switch {
	case title == "":
		return "title is required"
	case len(title) > maxPageTitleArgLength:
		return fmt.Sprintf("title is too long (maximum %d characters)", maxPageTitleArgLength)
	case slug == "":
		return "slug is required"
	case len(slug) > maxPageSlugArgLength:
		return fmt.Sprintf("slug is too long (maximum %d characters)", maxPageSlugArgLength)
	case len(parentID) > maxPageIDArgLength:
		return fmt.Sprintf("parentID is too long (maximum %d characters)", maxPageIDArgLength)
	}
	return ""
}
