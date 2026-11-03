package wiki

import (
	"context"
	stderrors "errors"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/http/dto"
	"github.com/perber/wiki/internal/search"
	"github.com/perber/wiki/internal/tags"
	wikimcp "github.com/perber/wiki/internal/wiki/mcp"
	wikipages "github.com/perber/wiki/internal/wiki/pages"
	wikisearch "github.com/perber/wiki/internal/wiki/search"
)

// WikiMCPAdapter implements mcp.PageOps by delegating to the existing
// pages/search use cases (ADR-0018). It is the only place the MCP slice meets
// the wiki's services: the slice never talks to internal/core or the
// top-level index packages directly, it calls PageOps, and this adapter fills
// in the slice's read-only DTOs.
type WikiMCPAdapter struct {
	tree        *tree.TreeService
	searchIndex *search.SQLiteIndex
	tags        *tags.TagsService
}

// NewWikiMCPAdapter constructs a WikiMCPAdapter over the wiki's services.
func NewWikiMCPAdapter(w *Wiki) *WikiMCPAdapter {
	return &WikiMCPAdapter{
		tree:        w.tree,
		searchIndex: w.searchIndex,
		tags:        w.tags,
	}
}

// GetPage reads a page and its metadata by stable ID, delegating to the
// existing get-page-by-ID use case.
func (a *WikiMCPAdapter) GetPage(ctx context.Context, id string) (*wikimcp.Page, error) {
	view, err := wikipages.NewGetPageViewByIDUseCase(a.tree).Execute(ctx, wikipages.GetPageViewByIDInput{ID: id})
	if err != nil {
		return nil, mapReadError(err)
	}
	return mcpPageFromView(view), nil
}

// GetPageByPath reads a page and its metadata by route path, delegating to the
// existing get-page-by-path use case.
func (a *WikiMCPAdapter) GetPageByPath(ctx context.Context, routePath string) (*wikimcp.Page, error) {
	view, err := wikipages.NewGetPageViewByPathUseCase(a.tree).Execute(ctx, wikipages.GetPageViewByPathInput{RoutePath: routePath})
	if err != nil {
		return nil, mapReadError(err)
	}
	return mcpPageFromView(view), nil
}

// mapReadError converts the tree sentinels the page-read use cases pass
// through into the LocalizedErrors the REST route would return for them
// (respondWithPageError), so the tools report the same client-facing codes
// and the tool error mapper does not fall back to mcp_internal_error. The
// other tree sentinels are not reachable from these read ops:
// ErrParentNotFound is only produced by mutations (create/move), and the
// search and tree-list ops do not surface a tree sentinel at all.
func mapReadError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case stderrors.Is(err, tree.ErrPageNotFound):
		return sharederrors.NewLocalizedError(wikipages.ErrCodePageNotFound, "Page not found", "page not found", err)
	case stderrors.Is(err, tree.ErrTreeNotLoaded):
		return sharederrors.NewLocalizedError(wikipages.ErrCodePageInternalError, "Tree not loaded", "tree not loaded", err)
	default:
		return err
	}
}

// Search runs a full-text search, delegating to the existing search use case.
func (a *WikiMCPAdapter) Search(ctx context.Context, query string, limit int) (*wikimcp.SearchResult, error) {
	out, err := wikisearch.NewSearchUseCase(a.searchIndex, a.tags, a.tree).Execute(
		ctx, wikisearch.SearchInput{Query: query, Limit: limit},
	)
	if err != nil {
		return nil, err
	}
	return mcpSearchFromResult(out.Result), nil
}

// ListTree returns the wiki's page tree (top-level nodes, synthetic root
// stripped), reading it straight from the tree like the pages route does.
func (a *WikiMCPAdapter) ListTree(_ context.Context) ([]wikimcp.TreeNode, error) {
	root := a.tree.GetTree()
	if root == nil {
		return nil, nil
	}
	return mcpTreeFromNodes(root.Children), nil
}

// mcpPageFromView maps an existing page view (node + content + metadata) onto
// the slice's read-only Page DTO.
func mcpPageFromView(view *wikipages.PageView) *wikimcp.Page {
	page := view.Page
	if page == nil || page.PageNode == nil {
		return nil
	}
	return &wikimcp.Page{
		ID:         page.ID,
		Title:      page.Title,
		Path:       dto.BuildPathFromNode(page.PageNode),
		Kind:       string(page.Kind),
		Content:    page.Content,
		Tags:       view.Tags,
		Properties: view.Properties,
		CreatedAt:  page.Metadata.CreatedAt,
		UpdatedAt:  page.Metadata.UpdatedAt,
	}
}

// mcpSearchFromResult maps a search result onto the slice's SearchResult DTO,
// keeping only the fields a client needs to act on a hit.
func mcpSearchFromResult(result *search.SearchResult) *wikimcp.SearchResult {
	if result == nil {
		return &wikimcp.SearchResult{}
	}
	items := make([]wikimcp.SearchItem, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, wikimcp.SearchItem{
			ID:      item.PageID,
			Title:   item.Title,
			Path:    item.Path,
			Kind:    item.Kind,
			Excerpt: item.Excerpt,
		})
	}
	return &wikimcp.SearchResult{Count: result.Count, Items: items}
}

// mcpTreeFromNodes maps a level of tree nodes (and their descendants) onto the
// slice's TreeNode DTOs, building each node's route path as it descends.
func mcpTreeFromNodes(nodes []*tree.PageNode) []wikimcp.TreeNode {
	result := make([]wikimcp.TreeNode, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		result = append(result, mcpTreeNodeFromNode(node, ""))
	}
	return result
}

func mcpTreeNodeFromNode(node *tree.PageNode, parentPath string) wikimcp.TreeNode {
	path := node.Slug
	if parentPath != "" {
		path = parentPath + "/" + node.Slug
	}
	children := make([]wikimcp.TreeNode, 0, len(node.Children))
	for _, child := range node.Children {
		if child == nil {
			continue
		}
		children = append(children, mcpTreeNodeFromNode(child, path))
	}
	return wikimcp.TreeNode{
		ID:       node.ID,
		Title:    node.Title,
		Slug:     node.Slug,
		Kind:     string(node.Kind),
		Path:     path,
		Children: children,
	}
}
