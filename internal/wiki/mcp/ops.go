package mcp

import (
	"context"
	"time"
)

// Page is the read-only view of a page that the page-read tools return. It
// mirrors the fields the existing page DTO surfaces (content, title, path,
// metadata) without the slice depending on other slices or on internal/core
// types (ADR-0018); the composition root's WikiMCPAdapter fills it in.
type Page struct {
	ID         string
	Title      string
	Path       string // route path (the wiki URL path), e.g. "docs/intro"
	Kind       string // "page" or "section"
	Content    string // markdown body
	Tags       []string
	Properties map[string]string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// SearchItem is one hit in a search result.
type SearchItem struct {
	ID      string
	Title   string
	Path    string
	Kind    string
	Excerpt string
}

// SearchResult is the output of the search operation.
type SearchResult struct {
	Count int
	Items []SearchItem
}

// TreeNode is one node in the wiki's page tree, with its children nested.
type TreeNode struct {
	ID       string
	Title    string
	Slug     string
	Kind     string
	Path     string
	Children []TreeNode
}

// PageOps is the narrow consumer interface the read tools delegate to. The
// slice never talks to the wiki's services directly: tool handlers bind their
// arguments, gate the caller's role, call one of these, format the result, and
// map errors onto ToolError. The composition root implements PageOps
// (WikiMCPAdapter) by delegating to the existing pages/search use cases, so
// write/revision/user tools in later PRs can add methods here without churn.
type PageOps interface {
	// GetPage reads a page and its metadata by stable page ID.
	GetPage(ctx context.Context, id string) (*Page, error)
	// GetPageByPath reads a page and its metadata by its route path.
	GetPageByPath(ctx context.Context, routePath string) (*Page, error)
	// Search runs a full-text search and returns the matching pages.
	Search(ctx context.Context, query string, limit int) (*SearchResult, error)
	// ListTree returns the wiki's page tree, top-level nodes first.
	ListTree(ctx context.Context) ([]TreeNode, error)
}
