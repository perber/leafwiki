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

// newListTreeTool registers the list_tree tool: it returns the wiki's page
// tree, every page and section with its ID, title, slug, kind and route path.
func newListTreeTool(log *slog.Logger, ops PageOps) mcpserver.ServerTool {
	tool := mcpgo.NewTool("list_tree",
		mcpgo.WithDescription("List the wiki's page tree. Returns every page and section with its ID, title, slug, kind and route path."),
	)
	return mcpserver.ServerTool{
		Tool: tool,
		Handler: func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			if toolErr := requireRole(ctx, coreauth.RoleViewer); toolErr != nil {
				return toolResult(log, tool.Name, toolErr), nil
			}
			nodes, err := ops.ListTree(ctx)
			if err != nil {
				return toolResult(log, tool.Name, toolErrorFrom(err)), nil
			}
			return mcpgo.NewToolResultText(renderTree(nodes)), nil
		},
	}
}

// renderTree formats the page tree as an indented list, one line per node.
// An empty tree is reported plainly.
func renderTree(nodes []TreeNode) string {
	if len(nodes) == 0 {
		return "The wiki is empty."
	}
	var sb strings.Builder
	for _, node := range nodes {
		writeTreeNode(&sb, node, 0)
	}
	return sb.String()
}

func writeTreeNode(sb *strings.Builder, node TreeNode, depth int) {
	sb.WriteString(strings.Repeat("  ", depth))
	fmt.Fprintf(sb, "- %s [%s] (%s) /%s\n", node.Title, node.ID, node.Kind, node.Path)
	for _, child := range node.Children {
		writeTreeNode(sb, child, depth+1)
	}
}
