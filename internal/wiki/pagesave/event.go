package pagesave

import "github.com/perber/wiki/internal/core/tree"

// PageOperationType identifies which page mutation triggered a save event.
type PageOperationType string

const (
	PageOperationCreate  PageOperationType = "create"
	PageOperationUpdate  PageOperationType = "update"
	PageOperationMove    PageOperationType = "move"
	PageOperationDelete  PageOperationType = "delete"
	PageOperationRestore PageOperationType = "restore"
)

// PageSaveEvent carries all context a side effect needs to react to a page mutation.
type PageSaveEvent struct {
	Operation PageOperationType
	UserID    string

	// Before is the page state prior to the operation; nil for Create.
	Before *tree.Page
	// After is the page state after the operation; nil for Delete.
	After *tree.Page

	ContentChanged bool
	SlugChanged    bool
	TitleChanged   bool

	// OldPath is the path of Before before the mutation (CalculatePath on a live node
	// returns the new path after UpdateNode/MoveNode mutates the tree in place).
	OldPath string

	// AffectedPages contains every page touched by the operation (e.g. a moved/deleted subtree).
	// For single-page operations it holds the one affected page.
	AffectedPages []*tree.Page

	// Summary is passed to content revisions (e.g. "page created", "page copied").
	Summary string
}
