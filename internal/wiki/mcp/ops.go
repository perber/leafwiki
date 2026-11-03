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

// Actor identifies the user a write operation is recorded for. The tool
// handlers fill it in from the caller in the request context (after the role
// gate has run, so it is always non-zero when a delegate call happens); the
// adapter passes it to the use cases that need an acting user. The role is
// deliberately not carried here: each tool gates it itself, and the use
// cases only record an ID.
type Actor struct {
	ID string
}

// CreatePageInput is the input of the create_page tool.
type CreatePageInput struct {
	Actor    Actor
	Title    string
	Slug     string  // optional; the adapter derives one from the title
	Kind     string  // "page" or "section"; empty means "page"
	ParentID *string // optional; nil or empty creates at the wiki root
	Content  string  // empty leaves the page without a body
}

// UpdatePageInput is the input of the update_page tool: a full replace of the
// page's title and/or content, with omitted fields keeping their current
// values.
type UpdatePageInput struct {
	Actor   Actor
	ID      string
	Title   string  // empty keeps the current title
	Content *string // nil keeps the current content
}

// PatchRange is one line-range operation of a patch_page patch. The line
// numbers are 1-based and inclusive, matching the line numbers get_page
// renders, so an agent can point straight at the lines it read.
type PatchRange struct {
	Start   int    // first line of the range
	End     int    // last line of the range
	Replace string // replacement text; empty deletes the range's lines
}

// PatchPageInput is the input of the patch_page tool.
type PatchPageInput struct {
	Actor  Actor
	ID     string
	Ranges []PatchRange
}

// DeletePageInput is the input of the delete_page tool; Recursive carries the
// REST delete's ?recursive= flag.
type DeletePageInput struct {
	Actor     Actor
	ID        string
	Recursive bool
}

// MovePageInput is the input of the move_page tool: the page's new route path
// (the last segment is the new slug, the remaining segments name the
// destination parent; an empty path moves the page to the wiki root).
type MovePageInput struct {
	Actor Actor
	ID    string
	Path  string
}

// PageOps is the narrow consumer interface the page tools delegate to. The
// slice never talks to the wiki's services directly: tool handlers bind their
// arguments, gate the caller's role, call one of these, format the result, and
// map errors onto ToolError. The composition root implements PageOps
// (WikiMCPAdapter) by delegating to the existing pages/search use cases, so
// revision/user tools in later PRs can add methods here without churn.
type PageOps interface {
	// GetPage reads a page and its metadata by stable page ID.
	GetPage(ctx context.Context, id string) (*Page, error)
	// GetPageByPath reads a page and its metadata by its route path.
	GetPageByPath(ctx context.Context, routePath string) (*Page, error)
	// Search runs a full-text search and returns the matching pages.
	Search(ctx context.Context, query string, limit int) (*SearchResult, error)
	// ListTree returns the wiki's page tree, top-level nodes first.
	ListTree(ctx context.Context) ([]TreeNode, error)
	// CreatePage creates a page or section and returns it.
	CreatePage(ctx context.Context, in CreatePageInput) (*Page, error)
	// UpdatePage fully replaces a page's title and/or content and returns it.
	UpdatePage(ctx context.Context, in UpdatePageInput) (*Page, error)
	// PatchPage applies a line-range patch to a page's content and returns it.
	PatchPage(ctx context.Context, in PatchPageInput) (*Page, error)
	// DeletePage deletes a page (optionally its subtree), like the REST delete.
	DeletePage(ctx context.Context, in DeletePageInput) error
	// MovePage moves a page to a new route path.
	MovePage(ctx context.Context, in MovePageInput) error
}
