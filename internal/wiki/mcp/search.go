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

// The search tool's result cap. A sensible default keeps a result set small
// enough to fit an agent's context window, and the hard cap stops a client
// from requesting an unbounded result.
const (
	searchDefaultLimit = 20
	searchMaxLimit     = 100
)

// maxSearchQueryArgLength bounds the query argument. A query is short text;
// without the cap an oversized body reaches the 1 MiB transport cap and
// fails as an opaque JSON-RPC parse error instead of a clean
// mcp_invalid_input the client can act on.
const maxSearchQueryArgLength = 20000

// newSearchTool registers the search tool: a full-text search across all
// pages, returning each match with its title, route path, kind and an excerpt.
func newSearchTool(log *slog.Logger, ops PageOps) mcpserver.ServerTool {
	tool := mcpgo.NewTool("search",
		mcpgo.WithDescription("Full-text search across all pages. Returns matching pages with title, route path, kind and an excerpt."),
		mcpgo.WithString("query",
			mcpgo.Required(),
			mcpgo.Description("The search query"),
		),
		mcpgo.WithInteger("limit",
			mcpgo.Description(fmt.Sprintf("Maximum number of results (default %d, max %d)", searchDefaultLimit, searchMaxLimit)),
		),
	)
	return mcpserver.ServerTool{
		Tool: tool,
		Handler: func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			query := strings.TrimSpace(req.GetString("query", ""))
			if toolErr := requireRole(ctx, coreauth.RoleViewer); toolErr != nil {
				return toolResult(log, tool.Name, toolErr), nil
			}
			if query == "" {
				return toolResult(log, tool.Name, &ToolError{Code: ErrCodeMCPInvalidInput, Message: "query is required"}), nil
			}
			if len(query) > maxSearchQueryArgLength {
				return toolResult(log, tool.Name, &ToolError{
					Code:    ErrCodeMCPInvalidInput,
					Message: fmt.Sprintf("query is too long (maximum %d characters)", maxSearchQueryArgLength),
				}), nil
			}
			limit := req.GetInt("limit", searchDefaultLimit)
			if limit <= 0 {
				limit = searchDefaultLimit
			}
			if limit > searchMaxLimit {
				limit = searchMaxLimit
			}
			res, err := ops.Search(ctx, query, limit)
			if err != nil {
				return toolResult(log, tool.Name, toolErrorFrom(err)), nil
			}
			return mcpgo.NewToolResultText(renderSearch(res)), nil
		},
	}
}

// renderSearch formats a search result as a numbered list of matches. An empty
// result is reported plainly so an agent knows the query ran and matched
// nothing. The header states both the total match count and the number of
// items actually shown, because the list is capped at the limit.
func renderSearch(res *SearchResult) string {
	if res == nil || len(res.Items) == 0 {
		return "No results found."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d results (showing %d):\n\n", res.Count, len(res.Items))
	for i, item := range res.Items {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, item.Title)
		fmt.Fprintf(&sb, "   ID: %s\n", item.ID)
		if item.Path != "" {
			fmt.Fprintf(&sb, "   Path: %s\n", item.Path)
		}
		if item.Excerpt != "" {
			fmt.Fprintf(&sb, "   Excerpt: %s\n", item.Excerpt)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
