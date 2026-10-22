package scopedread

import (
	"context"
	"errors"
	"time"

	"github.com/perber/wiki/internal/core/tree"
)

// PageView is what a scoped caller sees of a page. The fields are an
// allow-list: no author IDs or names, no slug or path (which would reveal the
// tree above the scope), no frontmatter, tags or properties. Adding a field
// exposes it to everyone holding a share link; TestPageView_FieldsAreAllowListed
// guards that.
type PageView struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Kind is "page" or "section". A plain string, so the shape seen by
	// scoped callers doesn't change with the core NodeKind type.
	Kind      string    `json:"kind"`
	Content   string    `json:"content"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// PageSource is what PageReader needs from the tree. *tree.TreeService
// implements it.
type PageSource interface {
	subtreeChecker
	GetPage(id string) (*tree.Page, error)
}

var _ PageSource = (*tree.TreeService)(nil)

// PageReader reads single pages inside a ReadScope.
type PageReader struct {
	src PageSource
}

// NewPageReader returns a PageReader over src.
func NewPageReader(src PageSource) *PageReader {
	return &PageReader{src: src}
}

// Page returns the page id if it is inside scope. Out-of-scope, unknown and
// deleted pages all return ErrNotFound; operational errors (tree not loaded,
// file read failure) are returned as they are so callers don't report them as
// a missing page. Those errors can contain file system paths: an HTTP caller
// must answer them with a generic error and never send err.Error() to a
// scoped (anonymous) client.
func (r *PageReader) Page(ctx context.Context, scope ReadScope, id string) (*PageView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := scope.validate(); err != nil {
		return nil, err
	}
	if err := scope.check(r.src, id); err != nil {
		return nil, err
	}

	page, err := r.src.GetPage(id)
	if errors.Is(err, tree.ErrPageNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &PageView{
		ID:        page.ID,
		Title:     page.Title,
		Kind:      string(page.Kind),
		Content:   page.Content,
		UpdatedAt: page.Metadata.UpdatedAt,
	}, nil
}
