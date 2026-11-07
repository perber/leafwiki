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

// maxPageTitleArgLength bounds the title argument. A title is short text;
// without the cap an oversized body reaches the transport cap and fails as
// an opaque JSON-RPC parse error instead of a clean mcp_invalid_input the
// client can act on.
const maxPageTitleArgLength = 256

// maxPageSlugArgLength bounds the slug argument the same way: a slug is a
// single short URL segment.
const maxPageSlugArgLength = 128

// newCreatePageTool registers the create_page tool: it creates a page or
// section (optionally under a parent) and returns it in the get_page result
// shape. The role gate mirrors the REST create route, which requires editor.
func newCreatePageTool(log *slog.Logger, ops PageOps) mcpserver.ServerTool {
	tool := mcpgo.NewTool("create_page",
		mcpgo.WithDescription("Create a new page or section and return it (metadata plus markdown body with line numbers). Requires editor or admin role."),
		mcpgo.WithString("title",
			mcpgo.Required(),
			mcpgo.Description("The title of the new page"),
		),
		mcpgo.WithString("slug",
			mcpgo.Description("The slug (URL segment). Omit to derive one from the title, deduplicated among siblings"),
		),
		mcpgo.WithString("parentID",
			mcpgo.Description("The ID of the parent section to create under. Omit to create at the wiki root"),
		),
		mcpgo.WithString("kind",
			mcpgo.Description("Node kind: 'page' (default) or 'section'"),
		),
		mcpgo.WithString("content",
			mcpgo.Description("The markdown body. Omit for an empty page"),
		),
	)
	return mcpserver.ServerTool{
		Tool: tool,
		Handler: func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			if toolErr := requireRole(ctx, coreauth.RoleEditor); toolErr != nil {
				return toolResult(log, tool.Name, toolErr), nil
			}
			title := strings.TrimSpace(req.GetString("title", ""))
			slug := strings.TrimSpace(req.GetString("slug", ""))
			parentID := strings.TrimSpace(req.GetString("parentID", ""))
			kind := strings.TrimSpace(req.GetString("kind", ""))
			if message := validateCreatePageArgs(title, slug, parentID, kind); message != "" {
				return toolResult(log, tool.Name, &ToolError{Code: ErrCodeMCPInvalidInput, Message: message}), nil
			}
			in := CreatePageInput{
				Actor:   actorFromContext(ctx),
				Title:   title,
				Slug:    slug,
				Kind:    kind,
				Content: req.GetString("content", ""),
			}
			if parentID != "" {
				in.ParentID = &parentID
			}
			page, err := ops.CreatePage(ctx, in)
			if err != nil {
				return toolResult(log, tool.Name, toolErrorFrom(err)), nil
			}
			return mcpgo.NewToolResultText(renderPage(page)), nil
		},
	}
}

// validateCreatePageArgs bounds the short arguments and checks the kind
// value, returning the client-facing message or "" when the arguments are
// usable. (Content is bounded by the transport's body cap, not per argument.)
func validateCreatePageArgs(title, slug, parentID, kind string) string {
	switch {
	case title == "":
		return "title is required"
	case len(title) > maxPageTitleArgLength:
		return fmt.Sprintf("title is too long (maximum %d characters)", maxPageTitleArgLength)
	case len(slug) > maxPageSlugArgLength:
		return fmt.Sprintf("slug is too long (maximum %d characters)", maxPageSlugArgLength)
	case len(parentID) > maxPageIDArgLength:
		return fmt.Sprintf("parentID is too long (maximum %d characters)", maxPageIDArgLength)
	case kind != "" && kind != "page" && kind != "section":
		return "kind must be 'page' or 'section'"
	}
	return ""
}
