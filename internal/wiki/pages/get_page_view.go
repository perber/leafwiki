package pages

import (
	"context"

	"github.com/perber/wiki/internal/core/tree"
)

// PageView is a page together with the metadata read from its frontmatter.
// Tags and Properties are never nil.
type PageView struct {
	Page       *tree.Page
	Tags       []string
	Properties map[string]string
}

// newPageView completes a page looked up in the tree with its metadata. The
// single-page reads of the API and SEO share it so both return the same view.
// It is not a visibility layer: other readers still use the tree directly.
func newPageView(t *tree.TreeService, page *tree.Page) *PageView {
	tags, properties := readPageMetadata(t, page.ID)
	return &PageView{Page: page, Tags: tags, Properties: properties}
}

// ─── GetPageViewByID ────────────────────────────────────────────────────────

// GetPageViewByIDInput is the input for GetPageViewByIDUseCase.
type GetPageViewByIDInput struct {
	ID string
}

// GetPageViewByIDUseCase reads a page and its metadata by stable page ID.
type GetPageViewByIDUseCase struct {
	tree *tree.TreeService
}

// NewGetPageViewByIDUseCase constructs a GetPageViewByIDUseCase.
func NewGetPageViewByIDUseCase(t *tree.TreeService) *GetPageViewByIDUseCase {
	return &GetPageViewByIDUseCase{tree: t}
}

// Execute returns the page view; tree.ErrPageNotFound is passed through.
func (uc *GetPageViewByIDUseCase) Execute(_ context.Context, in GetPageViewByIDInput) (*PageView, error) {
	page, err := uc.tree.GetPage(in.ID)
	if err != nil {
		return nil, err
	}
	return newPageView(uc.tree, page), nil
}

// ─── GetPageViewByPath ──────────────────────────────────────────────────────

// GetPageViewByPathInput is the input for GetPageViewByPathUseCase.
type GetPageViewByPathInput struct {
	// RoutePath is the URL route path of the page (e.g. "docs/api/intro").
	RoutePath string
}

// GetPageViewByPathUseCase reads a page and its metadata by route path.
type GetPageViewByPathUseCase struct {
	tree *tree.TreeService
}

// NewGetPageViewByPathUseCase constructs a GetPageViewByPathUseCase.
func NewGetPageViewByPathUseCase(t *tree.TreeService) *GetPageViewByPathUseCase {
	return &GetPageViewByPathUseCase{tree: t}
}

// Execute returns the page view; tree.ErrPageNotFound is passed through.
func (uc *GetPageViewByPathUseCase) Execute(_ context.Context, in GetPageViewByPathInput) (*PageView, error) {
	if in.RoutePath == "" {
		return nil, newPageMissingPathError()
	}
	page, err := uc.tree.FindPageByRoutePath(in.RoutePath)
	if err != nil {
		return nil, err
	}
	return newPageView(uc.tree, page), nil
}
