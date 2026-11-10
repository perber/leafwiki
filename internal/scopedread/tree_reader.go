package scopedread

import (
	"context"
	"errors"

	"github.com/perber/wiki/internal/core/tree"
)

// TreeNodeView is what a scoped caller sees of a node in the navigation. The
// fields are an allow-list like PageView's: no slug or path (which would
// reveal the tree above the scope), no position, pin state, authors or
// timestamps. TestTreeNodeView_FieldsAreAllowListed guards that.
type TreeNodeView struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Kind is "page" or "section", a plain string like PageView.Kind.
	Kind string `json:"kind"`
	// Children are in navigation order; never null.
	Children []*TreeNodeView `json:"children"`
}

// TreeSource is what TreeReader needs from the tree. *tree.TreeService
// implements it.
type TreeSource interface {
	SubtreeRoutes(id string) ([]tree.RouteEntry, error)
}

var _ TreeSource = (*tree.TreeService)(nil)

// TreeReader reads the navigation inside a ReadScope.
type TreeReader struct {
	src TreeSource
}

// NewTreeReader returns a TreeReader over src.
func NewTreeReader(src TreeSource) *TreeReader {
	return &TreeReader{src: src}
}

// Tree returns the scope as a tree: the scope root on top, nothing above or
// beside it. With IncludeDescendants it holds every page below the root as
// the tree is now; without, the root alone with no children. An unknown or
// deleted root is ErrNotFound; operational errors are returned as they are,
// with the same rule as PageReader.Page: never send their text to a scoped
// caller.
func (r *TreeReader) Tree(ctx context.Context, scope ReadScope) (*TreeNodeView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := scope.validate(); err != nil {
		return nil, err
	}

	entries, err := r.src.SubtreeRoutes(scope.RootID)
	if errors.Is(err, tree.ErrPageNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 || entries[0].ID != scope.RootID {
		return nil, ErrNotFound
	}
	if !scope.IncludeDescendants {
		entries = entries[:1]
	}

	// Entries come parent first, so each parent is in byID before its children.
	byID := make(map[string]*TreeNodeView, len(entries))
	var top *TreeNodeView
	for i, e := range entries {
		v := &TreeNodeView{ID: e.ID, Title: e.Title, Kind: string(e.Kind), Children: []*TreeNodeView{}}
		byID[e.ID] = v
		if i == 0 {
			top = v
			continue
		}
		// One locked walk lists every parent before its children, so the
		// lookup can't miss; if it ever did, dropping the entry shows less,
		// never more.
		if parent, ok := byID[e.ParentID]; ok {
			parent.Children = append(parent.Children, v)
		}
	}
	return top, nil
}
