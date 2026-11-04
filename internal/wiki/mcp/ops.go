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
// page's title, slug and/or content, with omitted fields keeping their current
// values.
type UpdatePageInput struct {
	Actor   Actor
	ID      string
	Title   string  // empty keeps the current title
	Slug    string  // empty keeps the current slug
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

// EnsurePathInput is the input of the ensure_path tool: it ensures a route
// path exists (missing segments become sections, existing nodes are reused)
// with the last segment taking the given kind.
type EnsurePathInput struct {
	Actor Actor
	Path  string
	Title string // title of the last segment (an existing node keeps its own)
	Kind  string // "page" (default) or "section"
}

// ConvertNodeInput is the input of the convert_node tool.
type ConvertNodeInput struct {
	Actor Actor
	ID    string
	Kind  string // target kind: "page" or "section"
}

// CopyPageInput is the input of the copy_page tool: it copies a page (and its
// assets) under a new title and slug, optionally under a parent — the REST
// copy route's single-page semantics (the children stay with the source).
type CopyPageInput struct {
	Actor    Actor
	SourceID string
	ParentID *string // nil copies at the wiki root
	Title    string
	Slug     string
}

// Revision is one entry in a page's revision history (the get_page_revisions
// output); author identity stays an ID — user resolution happens outside the
// slice (ADR-0018).
type Revision struct {
	ID        string
	Type      string // "content_update", "asset_update", "structure_update", "restore"
	AuthorID  string
	CreatedAt time.Time
	Title     string
	Path      string
	Summary   string
}

// RevisionAsset is one asset attached to a revision.
type RevisionAsset struct {
	Name      string
	SizeBytes int64
	MIMEType  string
}

// RevisionSnapshot is one revision with its content — the output of
// get_page_revision, the REST revision-snapshot shape without user resolution.
type RevisionSnapshot struct {
	Revision *Revision
	Content  string
	Assets   []RevisionAsset
}

// RevisionAssetChange is one asset difference between two revisions.
type RevisionAssetChange struct {
	Name   string
	Status string // "added", "removed" or "changed"
}

// RevisionComparison is the output of compare_page_revisions: both revision
// snapshots, whether the content differs, and the asset differences — the
// REST comparison shape.
type RevisionComparison struct {
	Base           *RevisionSnapshot
	Target         *RevisionSnapshot
	ContentChanged bool
	AssetChanges   []RevisionAssetChange
}

// ListRevisionsInput is the input of the get_page_revisions tool.
type ListRevisionsInput struct {
	ID     string
	Cursor string // "" starts at the newest
	Limit  int
}

// ListRevisionsResult is the output of the ListRevisions op.
type ListRevisionsResult struct {
	Revisions  []Revision
	NextCursor string
}

// GetRevisionInput is the input of the get_page_revision tool.
type GetRevisionInput struct {
	ID         string
	RevisionID string
}

// CompareRevisionsInput is the input of the compare_page_revisions tool.
type CompareRevisionsInput struct {
	ID               string
	BaseRevisionID   string
	TargetRevisionID string
}

// RestoreRevisionInput is the input of the restore_page_revision tool.
type RestoreRevisionInput struct {
	Actor      Actor
	ID         string
	RevisionID string
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
	// EnsurePath ensures a route path exists and returns the final node.
	EnsurePath(ctx context.Context, in EnsurePathInput) (*Page, error)
	// ConvertNode converts a page to a section or a section to a page.
	ConvertNode(ctx context.Context, in ConvertNodeInput) error
	// CopyPage copies a page (and its assets) to a new node and returns it.
	CopyPage(ctx context.Context, in CopyPageInput) (*Page, error)
	// ListRevisions returns a page's revision history, newest first, and the
	// cursor for the next page; empty when the page has no revisions.
	ListRevisions(ctx context.Context, in ListRevisionsInput) (*ListRevisionsResult, error)
	// GetRevision reads one revision's content and assets by page and revision ID.
	GetRevision(ctx context.Context, in GetRevisionInput) (*RevisionSnapshot, error)
	// CompareRevisions compares two revisions of one page.
	CompareRevisions(ctx context.Context, in CompareRevisionsInput) (*RevisionComparison, error)
	// RestoreRevision restores a page to a revision and returns the page.
	RestoreRevision(ctx context.Context, in RestoreRevisionInput) (*Page, error)
}
